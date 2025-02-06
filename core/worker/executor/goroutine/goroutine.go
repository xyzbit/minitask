package goroutine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/worker/executor"
)

type taskCtrl struct {
	stopCh   chan struct{}
	pauseCh  chan struct{}
	resumeCh chan struct{}
	exitCh   chan struct{}
	fn       BizLogic
}

type Executor struct {
	rw    sync.RWMutex
	ctrls map[string]*taskCtrl // task key <==> status 1: running 2: paused 3: stop

	taskrw     sync.RWMutex
	taskStatus map[string]model.TaskStatus

	resultChan  chan *model.TaskExecResult // send external notifications when execution status changes
	bizLogicNew func() BizLogic
}

type BizLogic func(params *model.TaskExecParam) (finished bool, err error)

func NewExecutor(new func() BizLogic) executor.Interface {
	return &Executor{
		ctrls:       make(map[string]*taskCtrl, 0),
		taskStatus:  make(map[string]model.TaskStatus, 0),
		resultChan:  make(chan *model.TaskExecResult, 10),
		bizLogicNew: new,
	}
}

func (e *Executor) Run(params *model.TaskExecParam) error {
	key := params.TaskKey
	if ctrl := e.getTaskCtrl(key); ctrl != nil {
		return errors.New("task already running")
	}

	e.setTaskStatus(key, model.TaskStatusRunning)
	e.initTaskCtrl(key)

	go func() {
		var err error
		defer func() {
			if err != nil {
				log.Error("%v", err)
				e.syncRunFinishResult(key, err)
			}
			e.delTaskCtrl(key)
		}()

		finishCh := make(chan struct{}, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("task %s panic: %v", key, r)
				}
				finishCh <- struct{}{}
			}()

			e.run(params)
		}()

		ctrl := e.getTaskCtrl(key)
		select {
		case <-ctrl.exitCh:
			err = errors.New("force exit")
			return
		case <-finishCh:
			return
		}
	}()

	e.syncRunResult(key)
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

func (e *Executor) List(ctx context.Context) ([]*model.TaskExecResult, error) {
	return e.listTaskStatus(), nil
}

func (e *Executor) ChangeResult() <-chan *model.TaskExecResult {
	return e.resultChan
}

func (e *Executor) run(params *model.TaskExecParam) {
	taskKey := params.TaskKey
	ctrl := e.getTaskCtrl(taskKey)
	for {
		select {
		case <-ctrl.stopCh:
			log.Debug("executor is stopped...")
			e.syncStopResult(taskKey)
			return
		case <-ctrl.pauseCh:
			log.Debug("executor is paused...")
			e.syncPauseResult(taskKey)
			select {
			case <-ctrl.stopCh:
				log.Debug("executor be stop in pause...")
				e.syncStopResult(taskKey)
				return
			case <-ctrl.resumeCh:
				log.Debug("executor be resume in pause...")
				e.syncRunResult(taskKey)
			}
		default:
			finished, err := ctrl.fn(params)
			if err != nil || finished {
				e.syncRunFinishResult(taskKey, err)
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
		fn:       e.bizLogicNew(),
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
