package executor

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

var executorFactory = make(map[string]CreateExecutor)

type CreateExecutor func() (Interface, error)

// TODO 注册时同时写到mysql中，scheduler 可以提前判断任务类型是否支持.
func RegisteFactory(taskType string, ce CreateExecutor) {
	executorFactory[taskType] = ce
}

func GetFactory(taskType string) (CreateExecutor, bool) {
	e, ok := executorFactory[taskType]
	return e, ok
}

type Result struct {
	IsPaused bool
	Err      error
}

type Interface interface {
	Execute(ctx context.Context, task *model.Task) Result
	Stop(ctx context.Context) error
	Pause(ctx context.Context) error
}
