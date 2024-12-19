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
	c := cache.NewThreadSafeMap[*model.Task]()
	reals, err := loader.List(context.Background())
	if err != nil {
		panic(err)
	}
	for _, r := range reals {
		c.Set(r.TaskKey, r)
	}
	return &Indexer{
		cache:  c,
		loader: loader,
		resync: resync,
	}
}

func (i *Indexer) SetAfterChange(f func(task *model.Task)) {
	i.afterChange = f
}

func (i *Indexer) GetRealTask(key string) *model.Task {
	t, _ := i.cache.Get(key)
	return t
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
		resultChan := i.loader.ResultChan()
		for new := range resultChan {
			ch <- new
		}
	}()

	for change := range ch {
		i.processTask(change)
	}
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

	// 同步到本地缓存
	if c.Status.IsFinalStatus() {
		i.cache.Delete(c.TaskKey)
	} else {
		i.cache.Set(c.TaskKey, c)
	}

	// 执行回调
	if i.afterChange != nil {
		i.afterChange(c)
	}
	return
}
