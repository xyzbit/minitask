package controller

import (
	"context"

	"github.com/xyzbit/minitaskx/core/worker/infomer"
)

type Worker struct {
	infomer *infomer.Indexer
}

func NewWorker(infomer *infomer.Infomer) *Worker {
}

func (c *Worker) Run(ctx context.Context) error {
}
