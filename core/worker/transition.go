package worker

import (
	"context"

	"github.com/pkg/errors"
	"github.com/xyzbit/minitaskx/core/executor"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/pkg/concurrency"
	"github.com/xyzbit/minitaskx/pkg/log"
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
	executor, err := createExecutor()
	if err != nil {
		return errors.Wrapf(err, "任务[%s] 类型[%s] 创建执行器失败", taskKey, task.Type)
	}

	w.executors.Store(taskKey, executor)
	w.setRealRunStatus(taskKey, model.TaskStatusRunning)

	concurrency.SafeGoWithRecoverFunc(
		func() {
			runStatus := model.TaskStatusSuccess
			result := executor.Execute(ctx, task)
			if result.Err != nil {
				runStatus = model.TaskStatusFailed
			}
			if result.IsPaused {
				runStatus = model.TaskStatusPaused
			}

			w.setRealRunStatus(taskKey, runStatus)
		}, func(error) {
			log.Error("任务[%s], 运行 panic, err: %v", taskKey, err)

			w.executors.Delete(taskKey)
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
	e, _ := exec.(executor.Interface)

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
	e, _ := exec.(executor.Interface)

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
