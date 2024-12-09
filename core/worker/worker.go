package controller

import (
	"context"

	"github.com/xyzbit/minitaskx/core/worker/infomer"
	"github.com/xyzbit/minitaskx/core/worker/syncer"
)

type Worker struct {
	infomer *infomer.Indexer
	syncer  *syncer.Syncer
}

func (c *Worker) Run(ctx context.Context) error {
	// start infomer to monitor task's real status
	go c.infomer.Run(ctx)

	for {
		select {
		case <-ctx.Done():
		}
	}
}
