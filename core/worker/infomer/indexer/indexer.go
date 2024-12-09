package infomer

import (
	"context"
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/internal/cache"
)

// Indexer will maintain cache of actual executor status
type Indexer struct {
	cache    *cache.ThreadSafeMap
	loader   realTaskLoader
	recorder recorder
}

func NewIndexer(
	loader realTaskLoader,
	recorder recorder,
	resync time.Duration,
) *Indexer {
	return &Indexer{
		cache:    cache.NewThreadSafeMap(),
		loader:   loader,
		recorder: recorder,
	}
}

func (i *Indexer) monitor(ctx context.Context) error {
	// watch task's changes of real status
	eventChan := i.loader.ResultChan()
	for task := range eventChan {
		if task == nil {
			log.Warn("[Infomer] loader.ResultChan() return nil")
			continue
		}
		// 1. sync to repo
		if err := i.recorder.UpdateTask(ctx, task); err != nil {
			log.Error("[Infomer] UpdateTask(%s) failed: %v", task.TaskKey, err)
		}
		// 2. sync to local cache
		if task.Status.IsFinalStatus() {
			i.cache.Delete(task.TaskKey)
		} else {
			i.cache.Update(task.TaskKey, task)
		}
	}
	return nil
}

func (i *Indexer) initial(ctx context.Context) error {
	tasks, err := i.loader.List(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		i.cache.Add(task.TaskKey, task)
	}
	return nil
}
