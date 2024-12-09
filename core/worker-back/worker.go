package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	stderr "errors"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/xyzbit/minitaskx/core/components/discover"
	"github.com/xyzbit/minitaskx/core/components/taskrepo"
	"github.com/xyzbit/minitaskx/core/model"
)

type Worker struct {
	id   string
	ip   string
	port int

	// 记录任务当前实际运行状态
	taskStatus sync.Map
	// 任务执行器，worker 会管理它们的生命周期
	// task_key -> executor
	executors            sync.Map
	executorResults      sync.Map
	executorInsideRunNum atomic.Int32

	discover discover.Interface
	taskRepo taskrepo.Interface

	opts *options
}

func NewWorker(
	id string,
	ip string,
	port int,
	discover discover.Interface,
	taskRepo taskrepo.Interface,
	opts ...Option,
) *Worker {
	o := newOptions(opts...)
	w := &Worker{
		id:       id,
		ip:       ip,
		port:     port,
		discover: discover,
		taskRepo: taskRepo,
		opts:     o,
	}

	w.initTransitionFuncs()
	return w
}

// 'Run' will run the worker program.
// You can control the program to exit gracefully through the context's 'cancel and timeout'.
func (w *Worker) Run(ctx context.Context) error {
	// run async data reporting gorutine
	go w.reportResourceUsage(w.opts.reportResourceInterval)
	go w.syncRealStatusToTaskRecord(w.opts.syncRealStatusInterval)

	// run the main executor control loop.
	// wait until context canceled
	return w.run(ctx)
}

func (w *Worker) run(ctx context.Context) error {
	// register instance
	metadata, err := w.generateInstanceMetadata()
	if err != nil {
		return fmt.Errorf("生成实例元数据失败: %v", err)
	}
	success, err := w.discover.Register(discover.Instance{
		Ip:       w.ip,
		Port:     uint64(w.port),
		Enable:   true,
		Healthy:  true,
		Metadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("注册实例失败: %v", err)
	}
	if !success {
		return fmt.Errorf("注册实例失败")
	}

	// generate instance id if not exist
	if w.id == "" {
		if err := w.setInstanceID(); err != nil {
			return fmt.Errorf("生成实例 ID 失败: %v", err)
		}
	}

	// start run
	w.opts.logger.Info("Worker[%s] 开始运行...", w.id)

	execTicker := time.NewTicker(w.opts.runInterval)
	for {
		select {
		case <-ctx.Done():
			// cancel signal watch.
			stopCtx := context.Background()
			// prepare timeout context if need.
			if w.opts.shutdownTimeout > 0 {
				var cancel context.CancelFunc
				stopCtx, cancel = context.WithTimeout(stopCtx, w.opts.shutdownTimeout)
				defer cancel()
			}
			// graceful shutdown.
			_, rerr := w.discover.UnRegister(discover.Instance{
				Ip:   w.ip,
				Port: uint64(w.port),
			})
			serr := w.shutdown(stopCtx)
			return stderr.Join(rerr, serr)
		case <-execTicker.C:
			w.loadAndRunTasks(ctx)
		}
	}
}

func (w *Worker) shutdown(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			waitNum := w.executorInsideRunNum.Load()
			if waitNum == 0 {
				w.opts.logger.Info("waiting for all executor goroutine to exit finshed!")
				return nil
			}
			w.opts.logger.Debug("waiting for all executor goroutine to exit, waitNum: %d", waitNum)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (w *Worker) loadAndRunTasks(ctx context.Context) {
	wantTaskRuns, err := w.loadRunnableTasks(ctx)
	if err != nil {
		w.opts.logger.Error("Worker[%s] 加载可执行任务失败: %v", w.id, err)
		return
	}
	w.opts.logger.Info("Worker[%s] 加载到 %d 个可执行任务", w.id, len(wantTaskRuns))

	for _, want := range wantTaskRuns {
		realRunStatus := w.getRealRunStatus(want.TaskKey)

		if noNeedStatusSync(realRunStatus, want.WantRunStatus) {
			w.opts.logger.Debug("Worker[%s] 任务[%s] 实际状态:%s 期望状态:%s 无需同步", w.id, want.TaskKey, realRunStatus, want.WantRunStatus)
			continue
		}
		exist, err := w.handleExecptionIfExist(ctx, want.TaskKey)
		if err != nil {
			w.opts.logger.Error("Worker[%s] 任务[%s] 处理异常失败: %v", w.id, want.TaskKey, err)
			continue
		}
		if exist {
			w.opts.logger.Debug("Worker[%s] 任务[%s] 异常状态处理成功", w.id, want.TaskKey)
			continue
		}

		runStatusSync, err := model.GetTaskStatusTransitionFunc(realRunStatus, want.WantRunStatus)
		if err != nil {
			w.opts.logger.Error("Worker[%s] 任务[%s] 获取同步状态函数失败: %v", w.id, want.TaskKey, err)
			continue
		}
		if err := runStatusSync(ctx, want.TaskKey); err != nil {
			w.opts.logger.Error("Worker[%s] 任务[%s] 同步状态失败: %v", w.id, want.TaskKey, err)
			continue
		}
	}
}

func (w *Worker) handleExecptionIfExist(ctx context.Context, taskKey string) (exist bool, err error) {
	real := w.getRealRunStatus(taskKey)
	if !real.IsExecptionStatus() {
		return false, nil
	}

	switch real {
	case model.TaskStatusExecptionRun:
		err = w.runTask(ctx, taskKey)
	case model.TaskStatusExecptionPause:
		err = w.pausedTask(ctx, taskKey)
	case model.TaskStatusExecptionStop:
		err = w.stopTask(ctx, taskKey)
	}

	return true, err
}

func (w *Worker) loadRunnableTasks(ctx context.Context) ([]*model.TaskRun, error) {
	taskRuns, err := w.taskRepo.ListRunnableTasks(ctx, w.id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// 补全任务状态
	w.taskStatus.Range(func(key, value interface{}) bool {
		if !lo.ContainsBy(taskRuns, func(wantTaskRun *model.TaskRun) bool {
			return wantTaskRun.TaskKey == key.(string)
		}) {
			taskRuns = append(taskRuns, &model.TaskRun{TaskKey: key.(string), WantRunStatus: model.TaskStatusNotExist})
		}
		return true
	})

	return taskRuns, nil
}

func (w *Worker) setInstanceID() error {
	// 获取当前实例的 InstanceId(注册存在延迟，重试获取)
	for i := 0; i < 10; i++ {
		if w.id != "" {
			break
		}

		time.Sleep(1 * time.Second)
		instances, err := w.discover.GetAvailableInstances()
		if err != nil {
			w.opts.logger.Error("获取实例列表失败: %v", err)
			continue
		}

		for _, instance := range instances {
			if instance.Ip == w.ip && instance.Port == uint64(w.port) {
				w.id = instance.InstanceId
				break
			}
		}
	}

	if w.id == "" {
		return fmt.Errorf("未找到当前实例的 InstanceId")
	}
	return nil
}

func (w *Worker) reportResourceUsage(interval time.Duration) {
	for {
		metadata, err := w.generateInstanceMetadata()
		if err != nil {
			w.opts.logger.Error("获取资源使用情况失败: %v", err)
			continue
		}

		err = w.discover.UpdateInstance(discover.Instance{
			Ip:       w.ip,
			Port:     uint64(w.port),
			Enable:   true,
			Healthy:  true,
			Metadata: metadata,
		})
		if err != nil {
			w.opts.logger.Error("更新服务实例元数据失败: %v", err)
		}

		time.Sleep(interval + time.Duration(rand.Intn(500))*time.Millisecond)
	}
}

func (w *Worker) syncRealStatusToTaskRecord(interval time.Duration) {
	for {
		realRunStatusMap := make(map[string]model.TaskStatus)

		// clone a snapshot
		w.taskStatus.Range(func(key, value any) bool {
			realRunStatusMap[key.(string)] = value.(model.TaskStatus)
			return true
		})

		for taskKey, status := range realRunStatusMap {
			w.opts.logger.Debug("任务[%s], 状态同步中, 运行状态为[%s]", taskKey, status)
			if !status.IsFinalStatus() {
				err := w.taskRepo.UpdateTaskStatus(context.Background(), taskKey, status)
				if err != nil {
					w.opts.logger.Error("update task status failed %+v", err)
				}
			} else {
				// in final status
				result := w.getExecutorResult(taskKey)
				if err := w.taskRepo.FinshTaskTX(context.Background(), taskKey, status, result); err != nil {
					w.opts.logger.Error("finish task failed %+v", err)
					continue
				}
				w.taskStatus.Delete(taskKey)
				w.executors.Delete(taskKey)
				w.executorResults.Delete(taskKey)
			}

			if isExecutorHasExited(status) {
				w.executorInsideRunNum.Add(-1)
			}
		}

		time.Sleep(interval + time.Duration(rand.Intn(500))*time.Millisecond)
	}
}

func (w *Worker) getExecutorResult(taskKey string) string {
	resultRaw := ""
	result, ok := w.executorResults.Load(taskKey)
	if ok {
		r, _ := json.Marshal(result)
		resultRaw = string(r)
	}
	return resultRaw
}

func (w *Worker) getRealRunStatus(taskKey string) model.TaskStatus {
	realTaskStatusAny, ok := w.taskStatus.Load(taskKey)
	if !ok {
		return model.TaskStatusNotExist
	}
	realRunStatus, _ := realTaskStatusAny.(model.TaskStatus)
	return realRunStatus
}

func (w *Worker) setRealRunStatus(taskKey string, wantRunStatus model.TaskStatus) {
	if wantRunStatus == model.TaskStatusNotExist {
		w.taskStatus.Delete(taskKey)
		return
	}
	w.taskStatus.Store(taskKey, wantRunStatus)
}

func (w *Worker) setRealRunStatusCAS(taskKey string, old, new model.TaskStatus) bool {
	if new == model.TaskStatusNotExist {
		return w.taskStatus.CompareAndDelete(taskKey, old)
	}
	return w.taskStatus.CompareAndSwap(taskKey, old, new)
}

func noNeedStatusSync(real, want model.TaskStatus) bool {
	return real == want ||
		real.IsFinalStatus() ||
		real.IsWaitStatus()
}
