package infomer

import (
	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
)

type ChangeConsumer interface {
	// Get get a change from queue.
	Get() (item model.Change, shutdown bool)
	// Done mark a change has been processed.
	Done(item model.Change)
}

type changeConsumer struct {
	i *Infomer
}

func (cc *changeConsumer) Get() (item model.Change, shutdown bool) {
	return cc.i.changeQueque.Get()
}

func (cc *changeConsumer) Done(item model.Change) {
	t, exist := cc.i.indexer.cache.Get(item.TaskKey)
	if !exist {
		log.Error("Done task not found: %s", item.TaskKey)
		return
	}
	t.Status = item.Real
	cc.i.indexer.cache.Update(item.TaskKey, item)
	cc.i.changeQueque.Done(item)
}
