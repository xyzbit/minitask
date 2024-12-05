package worker

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/worker/executor"
	"github.com/xyzbit/minitaskx/internal/concurrency"
)

func (w *Worker) initTransitionFuncs() {
	var (
		run        = w.runTask
		pause      = w.pausedTask
		stop       = w.stopTask
		setPaused  = w.setPausedStatus
		setSuccess = w.setSuccessStatus
		setFailed  = w.setFailedStatus
	)
	model.RegisterTransitionFunc(model.TaskStatusNotExist, model.TaskStatusRunning, run)
	model.RegisterTransitionFunc(model.TaskStatusNotExist, model.TaskStatusPaused, setPaused)
	model.RegisterTransitionFunc(model.TaskStatusNotExist, model.TaskStatusSuccess, setSuccess)
	model.RegisterTransitionFunc(model.TaskStatusNotExist, model.TaskStatusFailed, setFailed)
	model.RegisterTransitionFunc(model.TaskStatusRunning, model.TaskStatusPaused, pause)
	model.RegisterTransitionFunc(model.TaskStatusRunning, model.TaskStatusSuccess, stop)
	model.RegisterTransitionFunc(model.TaskStatusRunning, model.TaskStatusFailed, stop)
	model.RegisterTransitionFunc(model.TaskStatusPaused, model.TaskStatusRunning, run)
	model.RegisterTransitionFunc(model.TaskStatusPaused, model.TaskStatusSuccess, stop)
	model.RegisterTransitionFunc(model.TaskStatusPaused, model.TaskStatusFailed, stop)
}

func (w *Worker) runTask(ctx context.Context, taskKey string) error {
	task, err := w.taskRepo.GetTask(ctx, taskKey)
	if err != nil {
		return errors.WithStack(err)
	}
	createExecutor, exist := executor.GetFactory(task.Type)
	if !exist {
		return errors.Errorf("任务[%s] 类型[%s] 未找到执行器", taskKey, task.Type)
	}
	exe, err := createExecutor()
	if err != nil {
		return errors.Wrapf(err, "任务[%s] 类型[%s] 创建执行器失败", taskKey, task.Type)
	}

	w.executors.Store(taskKey, exe)
	w.setRealRunStatus(taskKey, model.TaskStatusRunning)
	if exe.InsideWorker() {
		w.executorInsideRunNum.Add(1)
	}

	concurrency.SafeGoWithRecoverFunc(
		func() {
			for {
				result := exe.Run(ctx, task)
				if result.Status == executor.ExecStatusError {
					w.opts.logger.Error("任务[%s], 执行异常待重试, err: %v", taskKey, result.Msg)
					continue
				}

				exist, runStatus := fromExecutorStatus(result.Status)
				if !exist {
					w.opts.logger.Error("任务[%s], 执行器返回异常状态[%s]不支持, 设置为失败", taskKey, result.Status)
				}

				w.setRealRunStatus(taskKey, runStatus)
				w.executorResults.Store(taskKey, result)
				return
			}
		}, func(error) {
			log.Error("任务[%s], 运行 panic, err: %v", taskKey, err)

			w.executors.Delete(taskKey)
			w.executorResults.Delete(taskKey)
			w.setRealRunStatus(taskKey, model.TaskStatusExecptionRun)
		},
	)

	return nil
}

func (w *Worker) pausedTask(ctx context.Context, taskKey string) error {
	realStatus := w.getRealRunStatus(taskKey)
	if realStatus.IsFinalStatus() {
		log.Info("任务[%s] 状态已经为终态[%s], 不能暂停", taskKey, realStatus)
		return nil
	}

	exec, ok := w.executors.Load(taskKey)
	if !ok {
		return errors.Errorf("任务[%s] 执行器不存在", taskKey)
	}
	e, _ := exec.(executor.Executor)

	swaped := w.setRealRunStatusCAS(taskKey, realStatus, model.TaskStatusWaitPaused)
	if !swaped {
		log.Info("任务[%s] 状态已经被其他程序修改为[%s], 不能暂停", taskKey, realStatus)
		return nil
	}
	concurrency.SafeGoWithRecoverFunc(func() {
		e.Pause(ctx)
	}, func(err error) {
		log.Error("任务[%s], 暂停 panic: %v", taskKey, err)
		w.setRealRunStatus(taskKey, model.TaskStatusExecptionPause)
	})
	return nil
}

func (w *Worker) stopTask(ctx context.Context, taskKey string) error {
	realStatus := w.getRealRunStatus(taskKey)
	if realStatus.IsFinalStatus() {
		log.Info("任务[%s] 状态已经为终态[%s], 不能停止", taskKey, realStatus)
		return nil
	}

	exec, ok := w.executors.Load(taskKey)
	if !ok {
		return errors.Errorf("任务[%s] 执行器不存在", taskKey)
	}
	e, _ := exec.(executor.Executor)

	swaped := w.setRealRunStatusCAS(taskKey, realStatus, model.TaskStatusWaitStop)
	if !swaped {
		log.Info("任务[%s] 状态已经被其他程序修改为[%s], 不能停止", taskKey, realStatus)
		return nil
	}
	w.taskStatus.Store(taskKey, model.TaskStatusWaitStop)
	concurrency.SafeGoWithRecoverFunc(func() {
		e.Stop(ctx)
	}, func(err error) {
		log.Error("任务[%s], 停止 panic: %v", taskKey, err)
		w.setRealRunStatus(taskKey, model.TaskStatusExecptionStop)
	})
	return nil
}

func (w *Worker) setPausedStatus(_ context.Context, taskKey string) error {
	w.setRealRunStatus(taskKey, model.TaskStatusPaused)
	return nil
}

func (w *Worker) setSuccessStatus(_ context.Context, taskKey string) error {
	w.setRealRunStatus(taskKey, model.TaskStatusSuccess)
	return nil
}

func (w *Worker) setFailedStatus(_ context.Context, taskKey string) error {
	w.setRealRunStatus(taskKey, model.TaskStatusFailed)
	return nil
}

// 是否执行者程序已经退出.
func isExecutorHasExited(taskStatus model.TaskStatus) bool {
	return slices.Contains([]model.TaskStatus{
		model.TaskStatusWaitPaused,
		model.TaskStatusShutdown,
		model.TaskStatusSuccess,
		model.TaskStatusFailed,
	}, taskStatus)
}

func fromExecutorStatus(s executor.ExecStatus) (bool, model.TaskStatus) {
	runStatus := model.TaskStatusFailed
	exist := true
	switch s {
	case executor.ExecStatusPaused:
		runStatus = model.TaskStatusPaused
	case executor.ExecStatusSuccess:
		runStatus = model.TaskStatusSuccess
	case executor.ExecStatusFail:
		runStatus = model.TaskStatusFailed
	case executor.ExecStatusShutdown:
		runStatus = model.TaskStatusShutdown
	default:
		exist = false
	}
	return exist, runStatus
}
