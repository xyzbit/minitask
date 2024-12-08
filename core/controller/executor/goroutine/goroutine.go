package goroutine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/controller/executor"
	"github.com/xyzbit/minitaskx/core/model"
)

var (
	_ executor.Executor = (*Executor)(nil)
)

type taskCtrl struct {
	stopCh     chan struct{}
	pauseCh    chan struct{}
	resumeCh   chan struct{}
	shutdownCh chan struct{}
}

type Executor struct {
	running atomic.Bool
	wg      sync.WaitGroup

	rw    sync.RWMutex
	ctrls map[string]*taskCtrl // task key <==> status 1: running 2: paused 3: stop

	taskrw sync.RWMutex
	tasks  map[string]*model.Task

	resultChan chan executor.Event // send external notifications when execution status changes
}

func NewExecutor() (*Executor, error) {
	return &Executor{
		ctrls:      make(map[string]*taskCtrl, 0),
		tasks:      make(map[string]*model.Task, 0),
		resultChan: make(chan executor.Event, 10),
	}, nil
}

func (e *Executor) Run(task *model.Task) error {
	key := task.TaskKey
	taskCtrl := e.getTaskCtrl(key)
	if taskCtrl != nil {
		return errors.New("task already running")
	}

	e.setTask(key, task)
	e.initTaskCtrl(key)

	e.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("task %s panic: %v", key, r)
				log.Error("%v", err)
				e.syncRunResult(key, err)
			}
			e.delTaskCtrl(key)
			e.wg.Done()
		}()
		e.run(key)
	}()

	return nil
}

func (e *Executor) Stop(taskKey string) error {
	ch := e.getTaskCtrl(taskKey)
	if ch == nil {
		return errors.New("stop need after run")
	}
	if len(ch.stopCh) > 0 {
		return nil
	}
	ch.stopCh <- struct{}{}
	return nil
}

func (e *Executor) Pause(taskKey string) error {
	ch := e.getTaskCtrl(taskKey)
	if ch == nil {
		return errors.New("pause need after run")
	}
	if len(ch.stopCh) > 0 {
		return nil
	}
	ch.pauseCh <- struct{}{}
	return nil
}

func (e *Executor) Resume(taskKey string) error {
	ch := e.getTaskCtrl(taskKey)
	if ch == nil {
		return errors.New("stop need after run")
	}
	if len(ch.resumeCh) > 0 {
		return nil
	}

	ch.resumeCh <- struct{}{}
	return nil
}

func (e *Executor) Shutdown(ctx context.Context) error {
	// send shutdown notify
	for _, ch := range e.ctrls {
		ch.shutdownCh <- struct{}{}
	}

	// wait all goroutine exit
	shutdownCh := make(chan struct{})
	go func() {
		e.wg.Wait()
		shutdownCh <- struct{}{}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-shutdownCh:
		return nil
	}
}

func (e *Executor) List(ctx context.Context) ([]*model.Task, error) {
	return e.listTasks(), nil
}

func (e *Executor) ResultChan() <-chan executor.Event {
	return e.resultChan
}

func (e *Executor) run(taskKey string) {
	ctrl := e.getTaskCtrl(taskKey)
	i := 0
	for {
		select {
		case <-ctrl.shutdownCh:
			log.Debug("executor be shutdown...")
			return
		case <-ctrl.stopCh:
			log.Debug("executor is stopped...")
			e.syncStopResult(taskKey)
			return
		case <-ctrl.pauseCh:
			log.Debug("executor is paused...")
			select {
			case <-ctrl.shutdownCh:
				log.Debug("executor be shutdown in pause...")
				return
			case <-ctrl.stopCh:
				log.Debug("executor be stop in pause...")
				e.syncStopResult(taskKey)
				return
			case <-ctrl.resumeCh:
				log.Debug("executor be resume in pause...")
				e.syncResumeResult(taskKey)
			}
		default:
			if i > 5 {
				e.syncRunResult(taskKey, nil)
				return
			}
			log.Info("executor is running(%d)...", i)
		}
		time.Sleep(time.Second * 3)
	}
}

func (e *Executor) initTaskCtrl(taskKey string) {
	e.rw.Lock()
	defer e.rw.Unlock()
	e.ctrls[taskKey] = &taskCtrl{
		stopCh:     make(chan struct{}, 1),
		pauseCh:    make(chan struct{}, 1),
		resumeCh:   make(chan struct{}, 1),
		shutdownCh: make(chan struct{}, 1),
	}
}

func (e *Executor) getTaskCtrl(taskKey string) *taskCtrl {
	e.rw.RLock()
	defer e.rw.RUnlock()
	return e.ctrls[taskKey]
}

func (e *Executor) delTaskCtrl(taskKey string) {
	e.rw.Lock()
	defer e.rw.Unlock()
	delete(e.ctrls, taskKey)
}
