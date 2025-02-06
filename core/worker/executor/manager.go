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

func (ge *Manager) List(ctx context.Context) ([]*model.TaskExecResult, error) {
	tasks := make([]*model.TaskExecResult, 0)
	for typ, e := range executors {
		ts, err := e.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range ts {
			t.TaskType = typ
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
		err = exe.Run(&model.TaskExecParam{
			TaskKey: change.TaskKey,
			BizID:   change.Task.BizID,
			BizType: change.Task.BizType,
			Payload: change.Task.Payload,
		})
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

func (ge *Manager) ChangeResult() <-chan *model.TaskExecResult {
	resultCh := make(chan *model.TaskExecResult, resultChBuffer)
	for typ, e := range executors {
		go func(e Interface, t string) {
			for event := range e.ChangeResult() {
				event.TaskType = t
				resultCh <- event
			}
		}(e, typ)
	}
	return resultCh
}
