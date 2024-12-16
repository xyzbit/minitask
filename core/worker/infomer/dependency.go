package infomer

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

// Obtain real execution task's info
type realTaskLoader interface {
	List(ctx context.Context) ([]*model.Task, error)
	ResultChan() <-chan *model.Task
}

type recorder interface {
	UpdateTask(ctx context.Context, task *model.Task) error
	// 获取可运行的任务
	ListRunnableTasks(ctx context.Context, workerID string) ([]*model.Task, error)
}
