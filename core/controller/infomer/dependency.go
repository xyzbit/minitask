package infomer

import (
	"context"

	"github.com/xyzbit/minitaskx/core/controller/executor"
	"github.com/xyzbit/minitaskx/core/model"
)

// Obtain real execution task's info
type realTaskLoader interface {
	List(ctx context.Context) ([]*model.Task, error)
	ResultChan() <-chan executor.Event
}

type recorder interface {
	UpdateTask(ctx context.Context, status *model.Task) error
}
