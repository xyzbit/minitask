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

type ExecStatus int

const (
	ExecStatusPaused   ExecStatus = 1
	ExecStatusSuccess  ExecStatus = 2
	ExecStatusFail     ExecStatus = 3
	ExecStatusError    ExecStatus = 4 // 该状态表示未知错误, 会进行重试
	ExecStatusShutdown ExecStatus = 5 // 该状态表示优雅退出, 仅在 InsideWorker() 为 true 需要处理.
)

type Result struct {
	Status ExecStatus
	Msg    string
}

type Interface interface {
	// Execute a task and return standard results after completion.
	// The executor running inside the worker program recommends processing ctx.Done for gracefully exit.
	Execute(ctx context.Context, task *model.Task) Result
	// Stop a task. The task will stop running and become terminated, and cannot be restarted.
	Stop(ctx context.Context) error
	// Pause a task, the task will stop running and become suspended, and can be run again;
	Pause(ctx context.Context) error
	// InsideWorker Whether it is running inside a Worker.
	InsideWorker() bool
}
