package infomer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/internal/queue"
	"github.com/xyzbit/minitaskx/pkg/util/retry"
)

type Infomer struct {
	running atomic.Bool

	indexer     *Indexer
	recorder    recorder
	changeQueue queue.TypedInterface[model.Change]

	logger log.Logger
}

func New(
	indexer *Indexer,
	recorder recorder,
	logger log.Logger,
) *Infomer {
	return &Infomer{
		indexer:     indexer,
		recorder:    recorder,
		changeQueue: queue.NewTyped[model.Change](),
		logger:      logger,
	}
}

func (i *Infomer) Run(ctx context.Context, workerID string, resync time.Duration) error {
	swapped := i.running.CompareAndSwap(false, true)
	if !swapped {
		return errors.New("infomer already running")
	}
	trigger, err := i.makeTigger(ctx, workerID, resync)
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
		i.changeQueue.ShutDownWithDrain()
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

// update recorder want + real cache need atomic.
func (i *Infomer) enqueueIfTaskChange(ctx context.Context, ch <-chan triggerInfo) {
	for {
		select {
		case <-ctx.Done():
			return
		case triggerInfo, ok := <-ch:
			if !ok {
				return
			}
			// load want and real task status
			taskPairs, err := i.loadTaskPairsThreadSafe(ctx, triggerInfo)
			if err != nil {
				i.logger.Error("[Infomer] loadTaskPairs failed: %v", err)
				continue
			}
			if len(taskPairs) == 0 {
				continue
			}

			// diff to get change
			changes := diff(taskPairs)

			// handle exception change.
			changes = i.handleException(changes)

			// changeQueue can ensure that only one operation of a task is executed at the same time.
			for _, change := range changes {
				if exist := i.changeQueue.Add(change); !exist {
					i.logger.Info("[Infomer] enqueue change: %v", change)
				}
			}
		}
	}
}

func (i *Infomer) monitorChangeResult(ctx context.Context) {
	i.indexer.SetAfterChange(func(t *model.TaskExecResult) {
		i.logger.Info("[Infomer] monitor task %s status changed: %s", t.TaskKey, t.Status)

		if err := retry.Do(func() error {
			return i.recorder.UpdateTask(context.Background(), &model.Task{
				TaskKey: t.TaskKey,
				Status:  t.Status,
			})
		}); err != nil {
			i.logger.Error("[Infomer] UpdateTask(%s) failed: %v", t.TaskKey, err)
		}

		// mark change done, other operation of the task can enqueue.
		i.changeQueue.Done(model.Change{TaskKey: t.TaskKey}) // only need task key to mask.
	})
	// monitor real task status
	i.indexer.Monitor(ctx)
}

type taskPair struct {
	want *model.Task
	real *model.TaskExecResult
}

func (i *Infomer) loadTaskPairsThreadSafe(ctx context.Context, info triggerInfo) ([]taskPair, error) {
	// 1. check processing task, Ensure serial execution of the same task.
	processingKeys := make(map[string]struct{}, len(info.taskKeys))
	for _, key := range info.taskKeys {
		if i.changeQueue.Exist(model.Change{TaskKey: key}) {
			processingKeys[key] = struct{}{}
		}
	}

	// 2. load want and real task.
	wantTaskKeys, realTaskKeys := info.taskKeys, info.taskKeys
	if info.resync {
		realTaskKeys = i.indexer.ListTaskKeys()
	}
	if len(processingKeys) > 0 {
		wtemp, rtemp := make([]string, 0, len(wantTaskKeys)), make([]string, 0, len(realTaskKeys))
		for _, key := range wantTaskKeys {
			if _, exist := processingKeys[key]; !exist {
				wtemp = append(wtemp, key)
			}
		}
		for _, key := range realTaskKeys {
			if _, exist := processingKeys[key]; !exist {
				rtemp = append(rtemp, key)
			}
		}
		wantTaskKeys, realTaskKeys = wtemp, rtemp
	}
	taskPairs, err := i.loadTaskPairs(ctx, wantTaskKeys, realTaskKeys)
	if err != nil {
		return nil, err
	}
	if len(taskPairs) == 0 {
		return nil, nil
	}

	// 3. filter finished task.
	// After the task is completed, the system will automatically modify the task state.
	// This action occurs in parallel with 'diff' logic.
	// So we filter out tasks that are completed.
	ret := make([]taskPair, 0, len(taskPairs))
	for _, pair := range taskPairs {
		if want := pair.want; want != nil {
			if want.Status.IsFinalStatus() {
				continue
			}
		}
		if real := pair.real; real != nil {
			if real.Status.IsFinalStatus() {
				continue
			}
		}
		ret = append(ret, pair)
	}
	return ret, nil
}

func (i *Infomer) loadTaskPairs(ctx context.Context, wantTaskKeys, realTaskKeys []string) ([]taskPair, error) {
	if len(wantTaskKeys) == 0 && len(realTaskKeys) == 0 {
		return nil, nil
	}

	wantTasks, err := i.recorder.BatchGetTask(ctx, wantTaskKeys) // 2.是不是延迟删除导致的，如果是要在diff判断状态
	if err != nil {
		return nil, err
	}
	realTasks := i.indexer.ListTasks(realTaskKeys)
	if len(wantTasks) == 0 && len(realTasks) == 0 {
		return nil, nil
	}

	realMap := lo.KeyBy(realTasks, func(t *model.TaskExecResult) string { return t.TaskKey })
	wantMap := lo.KeyBy(wantTasks, func(t *model.Task) string { return t.TaskKey })

	taskPairs := make([]taskPair, 0, len(wantTasks))
	for _, want := range wantTasks {
		taskPairs = append(taskPairs, taskPair{want: want, real: realMap[want.TaskKey]})
	}
	for _, real := range realTasks {
		_, exists := wantMap[real.TaskKey]
		if !exists {
			taskPairs = append(taskPairs, taskPair{real: real})
		}
	}

	return taskPairs, nil
}

func diff(taskPairs []taskPair) []model.Change {
	var changes []model.Change

	for _, pair := range taskPairs {
		change := model.Change{}
		want, real := pair.want, pair.real
		wantStatus, realStatus := model.TaskStatusNotExist, model.TaskStatusNotExist
		if real != nil {
			change.TaskKey = real.TaskKey
			change.TaskType = real.TaskType
			realStatus = real.Status
		}
		if want != nil {
			change.TaskKey = want.TaskKey
			change.TaskType = want.Type
			wantStatus = want.WantRunStatus
		}
		log.Debug("[Infomer] diff, want status: %v, real: %v", wantStatus, realStatus)

		if realStatus == wantStatus {
			continue
		}

		changeType, err := model.GetChangeType(realStatus, wantStatus)
		if err != nil {
			log.Error("[diff] task key: %s, realStatus: %s, wantStatus: %s, err: %v", change.TaskKey, realStatus, wantStatus, err)
			continue
		}
		change.ChangeType = changeType

		if changeType == model.ChangeCreate {
			change.Task = want
		}

		changes = append(changes, change)
	}

	return changes
}

func (i *Infomer) handleException(cs []model.Change) []model.Change {
	normalChanges := make([]model.Change, 0, len(cs))
	for _, c := range cs {
		if !c.IsException() {
			normalChanges = append(normalChanges, c)
			continue
		}

		if err := i.recorder.UpdateTask(context.Background(), &model.Task{
			TaskKey: c.TaskKey,
			Status:  model.TaskStatusPaused,
			Msg:     fmt.Sprintf("exception:%s", c.ChangeType),
		}); err != nil {
			log.Error("[Infomer] handleException task(%s), err: %v", c.TaskKey, err)
		}
	}
	return normalChanges
}
