package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/samber/lo"
	"github.com/xyzbit/minitaskx/core/executor"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/pkg/log"
)

const (
	statusRunning = 1
	statusPaused  = 2
	statusStop    = 3
)

type Executor struct {
	// 1: running 2: paused 3: stop
	status atomic.Int32
}

func NewExecutor() (executor.Interface, error) {
	return &Executor{}, nil
}

func (e *Executor) Execute(ctx context.Context, task *model.Task) executor.Result {
	status := e.status.Load()
	if status == statusRunning {
		return executor.Result{
			Status: executor.ExecStatusFail,
		}
	}
	e.status.Store(statusRunning)

	for {
		select {
		case <-ctx.Done():
			return executor.Result{
				Status: executor.ExecStatusShutdown,
			}
		default:
			s := e.status.Load()
			if s == statusStop || s == statusPaused {
				log.Info("executor is stop or paused...")
				return executor.Result{
					Status: lo.If(s == statusPaused, executor.ExecStatusPaused).Else(executor.ExecStatusSuccess),
				}
			}

			log.Info("executor is running...")
		}
		time.Sleep(time.Second * 3)
	}
}

func (e *Executor) Stop(ctx context.Context) error {
	e.status.Store(statusStop)
	return nil
}

func (e *Executor) Pause(ctx context.Context) error {
	e.status.Store(statusPaused)
	return nil
}

func (e *Executor) InsideWorker() bool {
	return true
}
