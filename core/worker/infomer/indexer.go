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
	resync time.Duration,
) *Indexer {
	i := &Indexer{
		loader: loader,
		resync: resync,
	}

	if err := i.initCache(); err != nil {
		panic(err)
	}
	return i
}

func (i *Indexer) SetAfterChange(f func(task *model.Task)) {
	i.afterChange = f
}

func (i *Indexer) ListTasks(keys []string) []*model.Task {
	list := i.cache.List()
	if len(keys) == 0 {
		return list
	}

	ret := make([]*model.Task, 0, len(keys))
	for _, item := range list {
		for _, key := range keys {
			if item.TaskKey == key {
				ret = append(ret, item)
			}
		}
	}
	return ret
}

// monitor real task status.
func (i *Indexer) Monitor(ctx context.Context) {
	ch := make(chan *model.Task, 100)

	// force cache refresh periodically
	go func() {
		ticker := time.NewTicker(i.resync)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				i.refreshCache(ctx, ch)
			}
		}
	}()
	// watch task's changes of real status
	go func() {
		resultChan := i.loader.ChangeResult()
		for new := range resultChan {
			ch <- new
		}
	}()

	for change := range ch {
		i.processTask(change)
	}
}

func (i *Indexer) initCache() error {
	recycleCondition := func(task *model.Task, afterSetDurtion time.Duration) bool {
		if task == nil {
			return true
		}
		b := task.Status.IsFinalStatus() && afterSetDurtion > time.Minute
		if b {
			log.Debug("[Infomer] recycle task: %s", task.TaskKey)
		}
		return b
	}

	c := cache.NewThreadSafeMap(recycleCondition)

	reals, err := i.loader.List(context.Background())
	if err != nil {
		return err
	}
	for _, r := range reals {
		c.Set(r.TaskKey, r)
	}

	i.cache = c
	return nil
}

func (i *Indexer) refreshCache(ctx context.Context, ch chan *model.Task) {
	newTasks, err := i.loader.List(ctx)
	if err != nil {
		log.Error("[Infomer] List() failed: %v", err)
		return
	}
	for _, new := range newTasks {
		old, exist := i.cache.Get(new.TaskKey)
		if !exist || new.Status != old.Status {
			ch <- new
		}
	}
}

func (i *Indexer) processTask(c *model.Task) {
	if c == nil {
		log.Error("[Infomer] received nil task")
		return
	}

	i.cache.Set(c.TaskKey, c)

	if i.afterChange != nil {
		i.afterChange(c)
	}
	return
}
