package goroutine

import (
	"github.com/xyzbit/minitaskx/core/model"
)

func (e *Executor) syncRunResult(taskKey string) {
	e.setTaskStatus(taskKey, model.TaskStatusRunning)
	e.resultChan <- &model.TaskExecResult{
		TaskKey: taskKey,
		Status:  model.TaskStatusRunning,
	}
}

func (e *Executor) syncRunFinishResult(taskKey string, err error) {
	result := &model.TaskExecResult{
		TaskKey: taskKey,
		Status:  model.TaskStatusSuccess,
	}
	if err != nil {
		result.Status = model.TaskStatusFailed
		result.Msg = err.Error()
	}
	e.setTaskStatus(taskKey, result.Status)
	e.resultChan <- result
}

func (e *Executor) syncPauseResult(taskKey string) {
	e.setTaskStatus(taskKey, model.TaskStatusPaused)
	e.resultChan <- &model.TaskExecResult{
		TaskKey: taskKey,
		Status:  model.TaskStatusPaused,
	}
}

func (e *Executor) syncStopResult(taskKey string) {
	e.setTaskStatus(taskKey, model.TaskStatusStop)
	e.resultChan <- &model.TaskExecResult{
		TaskKey: taskKey,
		Status:  model.TaskStatusStop,
	}
}

func (e *Executor) setTaskStatus(taskKey string, status model.TaskStatus) {
	e.taskrw.Lock()
	defer e.taskrw.Unlock()
	if status.IsFinalStatus() {
		delete(e.taskStatus, taskKey)
		return
	}
	e.taskStatus[taskKey] = status
}

func (e *Executor) listTaskStatus() []*model.TaskExecResult {
	e.taskrw.RLock()
	defer e.taskrw.RUnlock()
	tasks := make([]*model.TaskExecResult, 0, len(e.taskStatus))
	for key, t := range e.taskStatus {
		tasks = append(tasks, &model.TaskExecResult{
			TaskKey: key,
			Status:  t,
		})
	}
	return tasks
}
