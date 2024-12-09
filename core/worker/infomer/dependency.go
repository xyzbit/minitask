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

type recordUpdater interface {
	UpdateTask(ctx context.Context, status *model.Task) error
}

type recordLoader interface {
	// 获取可运行的任务
	ListRunnableTasks(ctx context.Context, workerID string) ([]*model.TaskRun, error)
}
