package infomer

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
)

// Infomer will maintain cache of actual executor status
type Infomer struct {
	running atomic.Bool

	cache    threadSafeMap
	loader   realTaskLoader
	recorder recorder

	resync time.Duration
}

func New(
	loader realTaskLoader,
	recorder recorder,
	resync time.Duration,
) *Infomer {
	if resync < time.Second { // min resync interval
		resync = time.Second
	}
	return &Infomer{
		cache:    threadSafeMap{items: make(map[string]interface{})},
		loader:   loader,
		recorder: recorder,
		resync:   resync,
	}
}

func (i *Infomer) Run(ctx context.Context) error {
	swapped := i.running.CompareAndSwap(false, true)
	if !swapped {
		return errors.New("infomer already running")
	}

	// sync all real task status
	if err := i.initialSync(ctx); err != nil {
		return err
	}
	// watch task's changes of real status
	eventChan := i.loader.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-eventChan:
			if !ok {
				return nil
			}
			// 1. sync to repo
			if err := i.recorder.UpdateTask(ctx, event.Task); err != nil {
				log.Error("[Infomer] UpdateTask(%s) failed: %v", event.Task.TaskKey, err)
			}
			// 2. sync to local cache
			if event.Task.Status.IsFinalStatus() {
				i.cache.Delete(event.Task.TaskKey)
			} else {
				i.cache.Update(event.Task.TaskKey, event.Task)
			}
		}
	}
}

func (i *Infomer) initialSync(ctx context.Context) error {
	tasks, err := i.loader.List(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		i.cache.Add(task.TaskKey, task)
	}
	return nil
}
