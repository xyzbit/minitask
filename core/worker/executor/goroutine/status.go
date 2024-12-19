package goroutine

import (
	"github.com/xyzbit/minitaskx/core/model"
)

func (e *Executor) syncRunResult(taskKey string) {
	cloneTask := e.getTask(taskKey)
	cloneTask.Status = model.TaskStatusRunning
	e.resultChan <- cloneTask
	e.setTask(taskKey, cloneTask)
}

func (e *Executor) syncRunFinshResult(taskKey string, err error) {
	cloneTask := e.getTask(taskKey)
	cloneTask.Status = model.TaskStatusSuccess
	if err != nil {
		cloneTask.Status = model.TaskStatusFailed
		cloneTask.Msg = err.Error()
	}
	e.resultChan <- cloneTask
	e.setTask(taskKey, cloneTask)
}

func (e *Executor) syncPauseResult(taskKey string) {
	cloneTask := e.getTask(taskKey)
	cloneTask.Status = model.TaskStatusPaused
	e.resultChan <- cloneTask
	e.setTask(taskKey, cloneTask)
}

func (e *Executor) syncStopResult(taskKey string) {
	cloneTask := e.getTask(taskKey)
	cloneTask.Status = model.TaskStatusStop
	e.resultChan <- cloneTask
	e.setTask(taskKey, cloneTask)
}

func (e *Executor) getTask(taskKey string) *model.Task {
	e.taskrw.RLock()
	defer e.taskrw.RUnlock()
	t, ok := e.tasks[taskKey]
	if !ok {
		return nil
	}
	return t.Clone()
}

func (e *Executor) setTask(taskKey string, task *model.Task) {
	e.taskrw.Lock()
	defer e.taskrw.Unlock()
	if task.Status.IsFinalStatus() {
		delete(e.tasks, taskKey)
		return
	}
	e.tasks[taskKey] = task
}

func (e *Executor) listTasks() []*model.Task {
	e.taskrw.RLock()
	defer e.taskrw.RUnlock()
	tasks := make([]*model.Task, 0)
	for _, t := range e.tasks {
		tasks = append(tasks, t.Clone())
	}
	return tasks
}
