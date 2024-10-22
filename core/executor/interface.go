package executor

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

var executorFactory = make(map[string]Factory)

type Factory interface {
	Create() (Interface, error)
}

func RegisteFactory(taskType string, factory Factory) {
	executorFactory[taskType] = factory
}

func GetFactory(taskType string) (Factory, bool) {
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
