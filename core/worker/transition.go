package worker

import (
	"context"
	"log"

	"github.com/pkg/errors"
	"github.com/xyzbit/minitaskx/core/executor"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/pkg/concurrency"
)

func (w *Worker) initTransitionFuncs() {
	var (
		run   = w.runTask
		pause = w.pausedTask
		stop  = w.stopTask
	)
	model.RegisterTransitionFunc(model.TaskStatusNotExist, model.TaskStatusRunning, run)
	model.RegisterTransitionFunc(model.TaskStatusRunning, model.TaskStatusPaused, pause)
	model.RegisterTransitionFunc(model.TaskStatusRunning, model.TaskStatusSuccess, stop)
	model.RegisterTransitionFunc(model.TaskStatusRunning, model.TaskStatusFailed, stop)
	model.RegisterTransitionFunc(model.TaskStatusPaused, model.TaskStatusRunning, run)
	model.RegisterTransitionFunc(model.TaskStatusPaused, model.TaskStatusSuccess, stop)
	model.RegisterTransitionFunc(model.TaskStatusPaused, model.TaskStatusFailed, stop)
}

func (w *Worker) runTask(ctx context.Context, taskKey string) error {
	task, err := w.taskRepo.GetTask(taskKey)
	if err != nil {
		return errors.WithStack(err)
	}
	executorFactory, exist := executor.GetFactory(task.Type)
	if !exist {
		return errors.Errorf("任务类型[%s] 未找到执行器", task.Type)
	}
	executor, err := executorFactory.Create()
	if err != nil {
		return errors.Wrapf(err, "任务类型[%s] 创建执行器失败", task.Type)
	}

	concurrency.SafeGoWithRecoverFunc(
		func() {
			w.setRealRunStatus(taskKey, model.TaskStatusRunning)

			runStatus := model.TaskStatusSuccess
			result := executor.Execute(ctx, task)
			if result.Err != nil {
				runStatus = model.TaskStatusFailed
			}
			if result.IsPaused {
				runStatus = model.TaskStatusPaused
			}

			w.setRealRunStatus(taskKey, runStatus)
		}, func(err error) {
			log.Printf("任务执行Panic, taskKey: %s, err: %v", taskKey, err)
			w.taskStatus.Delete(taskKey)
		},
	)

	return nil
}

func (w *Worker) pausedTask(ctx context.Context, taskKey string) error {
	exec, ok := w.executors.Load(taskKey)
	if !ok {
		return errors.Errorf("任务执行器不存在, taskKey: %s", taskKey)
	}
	e, _ := exec.(executor.Interface)

	concurrency.SafeGo(func() {
		e.Pause(ctx)
	})
	return nil
}

func (w *Worker) stopTask(ctx context.Context, taskKey string) error {
	exec, ok := w.executors.Load(taskKey)
	if !ok {
		return errors.Errorf("任务执行器不存在, taskKey: %s", taskKey)
	}
	e, _ := exec.(executor.Interface)
	concurrency.SafeGo(func() {
		e.Stop(ctx)
	})
	return nil
}
