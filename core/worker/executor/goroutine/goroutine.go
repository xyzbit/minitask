package goroutine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
)

type taskCtrl struct {
	stopCh   chan struct{}
	pauseCh  chan struct{}
	resumeCh chan struct{}
	exitCh   chan struct{}
}

type Executor struct {
	running atomic.Bool

	rw    sync.RWMutex
	ctrls map[string]*taskCtrl // task key <==> status 1: running 2: paused 3: stop

	taskrw sync.RWMutex
	tasks  map[string]*model.Task

	resultChan chan *model.Task // send external notifications when execution status changes
	fn         bizlogic
}

type bizlogic func(task *model.Task) (finshed bool, err error)

func NewExecutor(fn bizlogic) *Executor {
	return &Executor{
		ctrls:      make(map[string]*taskCtrl, 0),
		tasks:      make(map[string]*model.Task, 0),
		resultChan: make(chan *model.Task, 10),
		fn:         fn,
	}
}

func (e *Executor) Run(task *model.Task) error {
	key := task.TaskKey
	taskCtrl := e.getTaskCtrl(key)
	if taskCtrl != nil {
		return errors.New("task already running")
	}

	e.setTask(key, task)
	e.initTaskCtrl(key)

	go func() {
		var err error
		defer func() {
			if err != nil {
				log.Error("%v", err)
				e.syncRunResult(key, err)
			}
			e.delTaskCtrl(key)
		}()

		finshCh := make(chan struct{}, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("task %s panic: %v", key, r)
				}
				finshCh <- struct{}{}
			}()

			e.run(key)
		}()

		select {
		case <-taskCtrl.exitCh:
			err = fmt.Errorf("task %s force exit", key)
			return
		case <-finshCh:
			return
		}
	}()

	return nil
}

func (e *Executor) Exit(taskKey string) error {
	ch := e.getTaskCtrl(taskKey)
	if ch == nil {
		return errors.New("exit need after run")
	}
	if len(ch.stopCh) > 0 {
		return nil
	}
	ch.exitCh <- struct{}{}
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

func (e *Executor) List(ctx context.Context) ([]*model.Task, error) {
	return e.listTasks(), nil
}

func (e *Executor) ResultChan() <-chan *model.Task {
	return e.resultChan
}

func (e *Executor) run(taskKey string) {
	ctrl := e.getTaskCtrl(taskKey)
	for {
		select {
		case <-ctrl.stopCh:
			log.Debug("executor is stopped...")
			e.syncStopResult(taskKey)
			return
		case <-ctrl.pauseCh:
			log.Debug("executor is paused...")
			select {
			case <-ctrl.stopCh:
				log.Debug("executor be stop in pause...")
				e.syncStopResult(taskKey)
				return
			case <-ctrl.resumeCh:
				log.Debug("executor be resume in pause...")
				e.syncResumeResult(taskKey)
			}
		default:
			cloneTask := e.getTask(taskKey)
			finshed, err := e.fn(cloneTask)
			if err != nil {
				e.syncRunResult(taskKey, err)
				return
			}
			if finshed {
				e.syncRunResult(taskKey, nil)
				return
			}
		}
	}
}

func (e *Executor) initTaskCtrl(taskKey string) {
	e.rw.Lock()
	defer e.rw.Unlock()
	e.ctrls[taskKey] = &taskCtrl{
		stopCh:   make(chan struct{}, 1),
		pauseCh:  make(chan struct{}, 1),
		resumeCh: make(chan struct{}, 1),
		exitCh:   make(chan struct{}, 1),
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
