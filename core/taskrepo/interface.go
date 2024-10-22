package taskrepo

import (
	"github.com/xyzbit/minitaskx/core/model"
)

type Interface interface {
	// 事务创建任务记录和任务调度信息
	CreateTaskTX(task *model.Task, taskRun *model.TaskRun) error
	// 事务更新任务和任务调度信息
	UpdateTaskTX(task *model.Task, taskRun *model.TaskRun) error
	// 获取任务
	GetTask(taskKey string) (*model.Task, error)
	// 获取任务调度信息
	ListTaskRuns() ([]*model.TaskRun, error)
}
