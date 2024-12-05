package infomer

import (
	"context"
	"errors"
	"sync/atomic"
)

// Infomer will maintain cache of actual executor status
type Infomer struct {
	running atomic.Bool
	lw      ListAndWatcher
	cache   Cache
}

func New(lw ListAndWatcher, cache Cache) *Infomer {
	return &Infomer{
		lw:    lw,
		cache: cache,
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
	// watch task status
	eventChan := i.lw.ResultChan()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-eventChan:
			if !ok {
				return nil
			}
			i.cache.Update(event.Task.TaskKey, event.Task)
		}
	}
}

func (i *Infomer) initialSync(ctx context.Context) error {
	tasks, err := i.lw.List(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		i.cache.Add(task.TaskKey, task)
	}
	return nil
}
