package executor

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

const (
	resultChBuffer = 100
)

var executors = make(map[string]Interface)

func RegisterExecutor(taskType string, ce Interface) {
	executors[taskType] = ce
}

func GetExecutor(taskType string) (Interface, bool) {
	e, ok := executors[taskType]
	return e, ok
}

type Global struct{}

func (ge *Global) List(ctx context.Context) ([]*model.Task, error) {
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

func (ge *Global) ResultChan() <-chan *model.Task {
	resultCh := make(chan *model.Task, resultChBuffer)
	for _, e := range executors {
		go func(e Interface) {
			for event := range e.ResultChan() {
				resultCh <- event
			}
		}(e)
	}
	return resultCh
}
