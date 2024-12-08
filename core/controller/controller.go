package controller

import (
	"context"

	"github.com/xyzbit/minitaskx/core/controller/infomer"
	"github.com/xyzbit/minitaskx/core/controller/syncer"
)

type Controller struct {
	infomer *infomer.Infomer
	syncer  *syncer.Syncer
}

func (c *Controller) Run(ctx context.Context) error {
	//
	go c.infomer.Run(ctx)
	return nil
}
