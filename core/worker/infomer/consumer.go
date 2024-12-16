package infomer

import (
	"github.com/xyzbit/minitaskx/core/model"
)

type ChangeConsumer interface {
	// Pop get a change from queue.
	// if has no change, fuction will be blocked.
	WaitChange() (item model.Change, shutdown bool)
	JumpChange(item model.Change)
}

type changeConsumer struct {
	i *Infomer
}

func (cc *changeConsumer) WaitChange() (item model.Change, shutdown bool) {
	return cc.i.changeQueque.Get()
}

func (cc *changeConsumer) JumpChange(item model.Change) {
	cc.i.changeQueque.Done(item)
}
