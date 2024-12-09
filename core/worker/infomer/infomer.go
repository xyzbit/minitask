package infomer

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/xyzbit/minitaskx/core/model"
)

type Infomer struct {
	id      string
	running atomic.Bool

	indexer *Indexer
	loader  recordLoader

	opts *options
}

func New(id string, indexer *Indexer, loader recordLoader, opts ...Option) *Infomer {
	return &Infomer{
		id:      id,
		indexer: indexer,
		loader:  loader,
		opts:    newOptions(opts...),
	}
}

func (i *Infomer) Run(ctx context.Context) error {
	swapped := i.running.CompareAndSwap(false, true)
	if !swapped {
		return errors.New("infomer already running")
	}

	// init and monitor indexer
	if err := i.indexer.initAndMonitor(ctx); err != nil {
		return err
	}

	// compare task has changed
	execTicker := time.NewTicker(i.opts.runInterval)
	defer execTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			// cancel signal watch.
			stopCtx := context.Background()
			// prepare timeout context if need.
			if i.opts.shutdownTimeout > 0 {
				var cancel context.CancelFunc
				stopCtx, cancel = context.WithTimeout(stopCtx, w.opts.shutdownTimeout)
				defer cancel()
			}
			// graceful shutdown.
			return i.shutdown(stopCtx)
		case <-execTicker.C:
			i.enqueueIfTaskChange(ctx)
		}
	}
}

func (i *Infomer) shutdown(ctx context.Context) error {
}

func (i *Infomer) enqueueIfTaskChange(ctx context.Context) {
	wantTaskRuns, err := i.loadRunnableTasks(ctx)
	if err != nil {
		i.opts.logger.Error("Infomer[%s] 加载可执行任务失败: %v", i.id, err)
		return
	}
	i.opts.logger.Info("Worker[%s] 加载到 %d 个可执行任务", i.id, len(wantTaskRuns))
}

func (i *Infomer) loadRunnableTasks(ctx context.Context) ([]*model.TaskRun, error) {
	wantTaskRuns, err := i.loader.ListRunnableTasks(ctx, i.id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// complete task status
	// task 和 taskrun 需要一个统一的结构来承载数据，进行比较，思考一下（感觉）
	realTasks := i.indexer.listRealTasks()
	lo.Difference(wantTaskRuns, realTasks)

	return taskRuns, nil
}
