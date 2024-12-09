package executor

import (
	"context"

	"github.com/xyzbit/minitaskx/core/model"
)

// Executor is the interface of the executor.
type Executor interface {
	// (async) Run will create a executor's instance to run task and return standard results after completion.
	// The executor running inside the worker program recommends processing ctx.Done for gracefully exit.
	Run(task *model.Task) error
	// (async) Stop a executor. The task will stop running and become terminated, and cannot be restarted.
	Stop(taskKey string) error
	// (async) Pause a executor, the task will stop running and become suspended, and can be run again;
	Pause(taskKey string) error
	// (async) Resume a executor.
	Resume(taskKey string) error
	// Graceful exit, clean up and wait for resource reclamation and data synchronization to complete before exiting.
	// The timeout can be controlled through ctx.
	Shutdown(ctx context.Context) error

	List(ctx context.Context) ([]*model.Task, error)
	ResultChan() <-chan *model.Task
}
