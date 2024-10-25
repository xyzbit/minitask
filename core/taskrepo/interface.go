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
	// 更新任务信息
	UpdateTaskStatus(ctx context.Context, taskKey string, status model.TaskStatus) error
	// 获取任务
	GetTask(ctx context.Context, taskKey string) (*model.Task, error)
	// 批量获取任务
	BatchGetTask(ctx context.Context, taskKeys []string) ([]*model.Task, error)
	// 获任务调度信息
	ListTaskRuns(ctx context.Context) ([]*model.TaskRun, error)
	// 获取可运行的任务
	ListRunnableTasks(ctx context.Context, workerID string) ([]*model.TaskRun, error)
}
