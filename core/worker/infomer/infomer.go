package infomer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/internal/queue"
	"github.com/xyzbit/minitaskx/pkg/util/retry"
)

type Infomer struct {
	running atomic.Bool

	indexer      *Indexer
	recorder     recorder
	changeQueque queue.TypedInterface[model.Change]

	logger log.Logger
}

func New(
	indexer *Indexer,
	recorder recorder,
	logger log.Logger,
) *Infomer {
	return &Infomer{
		indexer:      indexer,
		recorder:     recorder,
		changeQueque: queue.NewTyped[model.Change](),
		logger:       logger,
	}
}

func (i *Infomer) Run(ctx context.Context, workerID string, runInterval time.Duration) error {
	swapped := i.running.CompareAndSwap(false, true)
	if !swapped {
		return errors.New("infomer already running")
	}
	trigger, err := i.monitorChangeTrigger(ctx, workerID, runInterval)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	// monitor change result
	wg.Add(1)
	go func() {
		defer wg.Done()
		i.monitorChangeResult(ctx)
	}()
	// compare task's change and enqueue.
	wg.Add(1)
	go func() {
		defer wg.Done()
		i.enqueueIfTaskChange(ctx, trigger)
	}()

	wg.Wait()
	return nil
}

func (i *Infomer) ChangeConsumer() ChangeConsumer {
	return &changeConsumer{i: i}
}

// graceful shutdown.
// Stop sending new events and wait for old events to be consumed.
func (i *Infomer) Shutdown(ctx context.Context) error {
	shutdownCh := make(chan struct{})
	go func() {
		i.changeQueque.ShutDownWithDrain()
		shutdownCh <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			i.logger.Error("[Infomer] shutdown timeout: %v", ctx.Err())
		}
		return ctx.Err()
	case <-shutdownCh:
		i.logger.Info("[Infomer] shutdown success")
		return nil
	}
}

// monitor changes in desired status
func (i *Infomer) monitorChangeTrigger(ctx context.Context, workerID string, runInterval time.Duration) (<-chan []string, error) {
	keysCh := make(chan []string, 100)
	ch, err := i.recorder.WatchRunnableTasks(ctx, workerID)
	if err != nil {
		return nil, err
	}
	go func() {
		for keys := range ch {
			keysCh <- keys
		}
	}()

	go func() {
		t := time.NewTicker(runInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				keys, err := i.recorder.ListRunnableTasks(context.Background(), workerID)
				if err != nil {
					i.logger.Error("[Infomer] monitorChangeWant ListRunnableTasks failed: %v", err)
					continue
				}
				keysCh <- keys
			}
		}
	}()

	return keysCh, nil
}

// update recorder want + real cache need atomic.
func (i *Infomer) enqueueIfTaskChange(ctx context.Context, keysCh <-chan []string) {
	for {
		select {
		case <-ctx.Done():
			return
		case keys, ok := <-keysCh:
			if !ok {
				return
			}

			for _, key := range keys {
				// check in queue.
				if i.changeQueque.Exist(model.Change{TaskKey: key}) {
					continue
				}

				// load want and real task status
				wantTask, err := i.recorder.GetTask(context.Background(), key)
				if err != nil {
					i.logger.Error("[Infomer] load task failed: %v", err)
					continue
				}
				realTask := i.indexer.GetRealTask(key)

				// diff to get change
				change := diff(wantTask, realTask)
				if change == nil {
					continue
				}

				// enqueue
				if exist := i.changeQueque.Add(*change); !exist {
					i.logger.Info("[Infomer] enqueue change: %v", change)
				}
			}
		}
	}
}

func (i *Infomer) monitorChangeResult(ctx context.Context) {
	i.indexer.SetAfterChange(func(t *model.Task) {
		i.logger.Info("[Infomer] monitor task %s status changed: %s", t.TaskKey, t.Status)

		if err := retry.Do(func() error {
			return i.recorder.UpdateTask(context.Background(), t)
		}); err != nil {
			i.logger.Error("[Infomer] UpdateTask(%s) failed: %v", t.TaskKey, err)
		}

		i.changeQueque.Done(model.Change{TaskKey: t.TaskKey}) // only need task key to mask.
	})
	// monitor real task status
	i.indexer.Monitor(ctx)
}

func diff(want *model.Task, real *model.Task) *model.Change {
	if want == nil && real == nil {
		return nil
	}

	change := &model.Change{}
	realStatus := model.TaskStatusNotExist
	wantStatus := model.TaskStatusNotExist
	if real != nil {
		realStatus = real.Status
		change.TaskKey = real.TaskKey
		change.TaskType = real.Type
		change.Task = real
	}
	if want != nil {
		wantStatus = want.Status
		change.TaskKey = want.TaskKey
		change.TaskType = want.Type
		change.Task = want
	}

	if realStatus == wantStatus {
		return nil
	}
	changeType, err := model.GetChangeType(realStatus, wantStatus)
	if err != nil {
		log.Error("[diff] task key: %s, realStatus: %s, wantStatus: %s, err: %v", want.TaskKey, realStatus, want.Status, err)
		return nil
	}
	change.ChangeType = changeType

	return change
}
