package executor

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

const (
	resultChBuffer = 16
)

var executors = make(map[string]Executor)

// TODO 注册时同时写到mysql中，scheduler 可以提前判断任务类型是否支持.
func RegisterExecutor(taskType string, ce Executor) {
	executors[taskType] = ce
}

func GetExecutor(taskType string) (Executor, bool) {
	e, ok := executors[taskType]
	return e, ok
}

func List(ctx context.Context) ([]*model.Task, error) {
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

func ResultChan() <-chan *model.Task {
	resultCh := make(chan *model.Task, resultChBuffer)
	for _, e := range executors {
		go func(e Executor) {
			for event := range e.ResultChan() {
				resultCh <- event
			}
		}(e)
	}
	return resultCh
}
