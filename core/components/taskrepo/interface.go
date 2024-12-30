package taskrepo

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

type Interface interface {
	// 事务创建任务记录和任务调度信息
	CreateTask(ctx context.Context, task *model.Task) error
	// 事务更新任务和任务调度信息
	UpdateTask(ctx context.Context, task *model.Task) error
	// 获取任务
	GetTask(ctx context.Context, taskKey string) (*model.Task, error)
	// 批量获取任务
	BatchGetTask(ctx context.Context, taskKeys []string) ([]*model.Task, error)
	// 查询任务列表
	ListTask(ctx context.Context, filter *model.TaskFilter) ([]*model.Task, error)

	// returns all runnable tasks of the current worker.
	// if workerID is empty, returns all runnable tasks.
	ListRunnableTasks(ctx context.Context, workerID string) (keys []string, err error)
	// watch all runnable tasks change.
	WatchRunnableTasks(ctx context.Context, workerID string) (keys <-chan []string, err error)
}
