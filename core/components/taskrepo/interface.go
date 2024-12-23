package taskrepo

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

type Interface interface {
	// 事务创建任务记录和任务调度信息
	CreateTaskTX(ctx context.Context, task *model.Task, taskRun *model.TaskRun) error
	// 事务更新任务和任务调度信息
	UpdateTaskTX(ctx context.Context, task *model.Task, taskRun *model.TaskRun) error
	// 获取任务
	GetTask(ctx context.Context, taskKey string) (*model.Task, error)
	// 批量获取任务
	BatchGetTask(ctx context.Context, taskKeys []string) ([]*model.Task, error)
	// 查询任务列表
	ListTask(ctx context.Context, filter *model.TaskFilter) ([]*model.Task, error)
	// 更新任务状态
	UpdateTask(ctx context.Context, task *model.Task) error
	// 完成任务
	FinishTask(ctx context.Context, task *model.Task) error

	// 批量获取任务
	BatchGetWantTask(ctx context.Context, taskKeys []string) ([]*model.Task, error)
	// 获任务调度信息
	ListTaskRuns(ctx context.Context) ([]*model.TaskRun, error)
	// returns all runnable tasks of the current worker.
	ListRunnableTasks(ctx context.Context, workerID string) (keys []string, err error)
	// watch all runnable tasks change.
	WatchRunnableTasks(ctx context.Context, workerID string) (keys <-chan []string, err error)
}
