package infomer

import (
	"context"
	"time"
)

const defaultResync = 15 * time.Second

type triggerInfo struct {
	resync   bool
	taskKeys []string
}

// watch trigger event.
func (i *Infomer) makeTigger(ctx context.Context, workerID string, resync time.Duration) (<-chan triggerInfo, error) {
	if resync <= 0 {
		resync = defaultResync
	}

	tasksCh := make(chan triggerInfo, 100)
	// init tasks.
	taskKeys, err := i.recorder.ListRunnableTasks(ctx, workerID)
	if err != nil {
		close(tasksCh)
		return nil, err
	}
	tasksCh <- triggerInfo{resync: true, taskKeys: taskKeys}

	// watch task change.
	ch, err := i.recorder.WatchRunnableTasks(ctx, workerID)
	if err != nil {
		return nil, err
	}
	go func() {
		for keys := range ch {
			tasksCh <- triggerInfo{resync: false, taskKeys: keys}
		}
	}()

	// resync task.
	go func() {
		ticker := time.NewTicker(resync)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				keys, err := i.recorder.ListRunnableTasks(context.Background(), workerID)
				if err != nil {
					i.logger.Error("[Infomer] monitorChangeWant ListRunnableTasks failed: %v", err)
					continue
				}
				tasksCh <- triggerInfo{resync: true, taskKeys: keys}
			}
		}
	}()

	return tasksCh, nil
}
