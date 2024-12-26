package executor

import (
	"context"
	"fmt"

	"github.com/xyzbit/minitaskx/core/model"
)

const (
	resultChBuffer = 100
)

var executors = make(map[string]Interface)

func RegisterExecutor(taskType string, ce Interface) {
	executors[taskType] = ce
}

func getExecutor(taskType string) (Interface, bool) {
	e, ok := executors[taskType]
	return e, ok
}

type Manager struct{}

func (ge *Manager) List(ctx context.Context) ([]*model.Task, error) {
	tasks := make([]*model.Task, 0)
	for _, e := range executors {
		ts, err := e.List(ctx)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, ts...)
	}
	return tasks, nil
}

func (ge *Manager) ChangeHandle(change *model.Change) error {
	exe, exist := getExecutor(change.TaskType)
	if !exist {
		return fmt.Errorf("executor type(%s)  not found", change.TaskType)
	}

	var err error
	switch change.ChangeType {
	case model.ChangeCreate:
		err = exe.Run(change.Task)
	case model.ChangeDelete:
		err = exe.Exit(change.TaskKey)
	case model.ChangePause:
		err = exe.Pause(change.TaskKey)
	case model.ChangeResume:
		err = exe.Resume(change.TaskKey)
	case model.ChangeStop:
		err = exe.Stop(change.TaskKey)
	default:
		err = fmt.Errorf("unknown change type: %s", change.ChangeType)
	}
	return err
}

func (ge *Manager) ChangeResult() <-chan *model.Task {
	resultCh := make(chan *model.Task, resultChBuffer)
	for _, e := range executors {
		go func(e Interface) {
			for event := range e.ChangeResult() {
				resultCh <- event
			}
		}(e)
	}
	return resultCh
}
