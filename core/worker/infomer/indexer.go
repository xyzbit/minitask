package infomer

import (
	"context"
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/internal/cache"
)

// Indexer will maintain cache of actual executor status
type Indexer struct {
	cache       *cache.ThreadSafeMap[*model.Task]
	loader      realTaskLoader
	afterChange func(task *model.Task)
	resync      time.Duration
}

func NewIndexer(
	loader realTaskLoader,
	afterChange func(task *model.Task),
	resync time.Duration,
) *Indexer {
	c := cache.NewThreadSafeMap[*model.Task]()
	tasks, err := loader.List(context.Background())
	if err != nil {
		panic(err)
	}
	for _, task := range tasks {
		c.Add(task.TaskKey, task)
	}
	return &Indexer{
		cache:       c,
		loader:      loader,
		afterChange: afterChange,
		resync:      resync,
	}
}

// Start init and start a goroutine to monitor real task status.
func (i *Indexer) monitor(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(i.resync)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tasks, err := i.loader.List(ctx)
				if err != nil {
					log.Error("[Infomer] List() failed: %v", err)
					continue
				}
				for _, task := range tasks {
					i.cache.Add(task.TaskKey, task)
				}
				// TODO to a channel
			}
		}
	}()

	// watch task's changes of real status
	go func() {
		eventChan := i.loader.ResultChan()
		// TODO to a channel
		for task := range eventChan {
			// mark queue as
		}
	}()

	for task := range channel {
		if task == nil {
			log.Warn("[Infomer] loader.ResultChan() return nil")
			continue
		}
		// // sync to repo
		// if err := i.recorder.UpdateTask(ctx, task); err != nil {
		// 	log.Error("[Infomer] UpdateTask(%s) failed: %v", task.TaskKey, err)
		// }
		i.afterChange(task)
		// sync to local cache
		if task.Status.IsFinalStatus() {
			i.cache.Delete(task.TaskKey)
		} else {
			i.cache.Update(task.TaskKey, task)
		}
	}
	return nil
}

func (i *Indexer) listRealTasks() []*model.Task {
	return i.cache.List()
}
