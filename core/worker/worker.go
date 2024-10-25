package worker

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/xyzbit/minitaskx/core/discover"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/taskrepo"
	"github.com/xyzbit/minitaskx/pkg/log"
)

type Worker struct {
	id   string
	ip   string
	port int

	// 记录任务当前实际运行状态
	// TODO 这里使用syncmap 效率较低读写平均，task_key -> run_status
	taskStatus sync.Map
	// 任务执行器，worker 会管理它们的生命周期
	// task_key -> executor
	executors sync.Map

	discover discover.Interface
	taskRepo taskrepo.Interface

	logger log.Logger
}

func NewWorker(
	id string,
	ip string,
	port int,
	discover discover.Interface,
	taskRepo taskrepo.Interface,

	logger log.Logger,
) *Worker {
	w := &Worker{
		id:       id,
		ip:       ip,
		port:     port,
		discover: discover,
		taskRepo: taskRepo,
		logger:   logger,
	}

	w.initTransitionFuncs()
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	metadata, err := generateInstanceMetadata()
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
	defer func() {
		_, err := w.discover.UnRegister(discover.Instance{
			Ip:   w.ip,
			Port: uint64(w.port),
		})
		if err != nil {
			w.logger.Error("注销实例失败: %v", err)
		}
	}()

	if w.id == "" {
		err = w.setInstanceID()
		if err != nil {
			return fmt.Errorf("设置实例 ID 失败: %v", err)
		}
	}

	go w.reportResourceUsage(5 * time.Second)
	go w.syncRealStatusToTaskRecord(3 * time.Second)

	return w.run(ctx)
}

func (w *Worker) run(ctx context.Context) error {
	w.logger.Info("开始运行...", w.id)

	execTicker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-execTicker.C:
			wantTaskRuns, err := w.loadRunnableTasks(ctx)
			if err != nil {
				w.logger.Error("加载可执行任务失败: %v", w.id, err)
				continue
			}
			w.logger.Info("加载到 %d 个可执行任务", w.id, len(wantTaskRuns))

			for _, want := range wantTaskRuns {
				realRunStatus := w.getRealRunStatus(want.TaskKey)

				if noNeedStatusSync(realRunStatus, want.WantRunStatus) {
					continue
				}
				exist, err := w.handleExecptionIfExist(ctx, want.TaskKey)
				if err != nil {
					w.logger.Error("任务[%s] 处理异常失败: %v", w.id, want.TaskKey, err)
					continue
				}
				if exist {
					continue
				}

				runStatusSync, err := model.GetTaskStatusTransitionFunc(realRunStatus, want.WantRunStatus)
				if err != nil {
					w.logger.Error("任务[%s] 获取同步状态函数失败: %v", w.id, want.TaskKey, err)
					continue
				}
				if err := runStatusSync(ctx, want.TaskKey); err != nil {
					w.logger.Error("任务[%s] 同步状态失败: %v", w.id, want.TaskKey, err)
					continue
				}
			}
		}
	}
}

func noNeedStatusSync(real, want model.TaskStatus) bool {
	return real == want ||
		real.IsFinalStatus() ||
		real.IsWaitStatus()
}

func (w *Worker) handleExecptionIfExist(ctx context.Context, taskKey string) (exist bool, err error) {
	real := w.getRealRunStatus(taskKey)
	if !real.IsExecptionStatus() {
		return false, nil
	}
	w.logger.Info("任务[%s] 处理状态异常: %s", taskKey, real)

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
		time.Sleep(500 * time.Millisecond)
		instances, err := w.discover.GetAvailableInstances()
		if err != nil {
			return fmt.Errorf("获取实例列表失败: %v", err)
		}

		for _, instance := range instances {
			if instance.Ip == w.ip && instance.Port == uint64(w.port) {
				w.id = instance.InstanceId
				break
			}
		}
		if w.id != "" {
			break
		}
	}

	if w.id == "" {
		return fmt.Errorf("未找到当前实例的 InstanceId")
	}
	return nil
}

func (w *Worker) reportResourceUsage(interval time.Duration) {
	for {
		metadata, err := generateInstanceMetadata()
		if err != nil {
			w.logger.Error("获取资源使用情况失败: %v", err)
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
			w.logger.Error("更新服务实例元数据失败: %v", err)
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
			err := w.taskRepo.UpdateTaskStatus(context.Background(), taskKey, status)
			if err != nil {
				w.logger.Error("update task status failed %+v", err)
			}
		}

		time.Sleep(interval + time.Duration(rand.Intn(500))*time.Millisecond)
	}
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
