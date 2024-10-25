package model

import (
	"context"
	"strings"

	"github.com/pkg/errors"
)

type TaskStatus string

const (
	TaskStatusNotExist       TaskStatus = "not_exist"
	TaskStatusWaitScheduling TaskStatus = "wait_scheduling"
	TaskStatusWaitRunning    TaskStatus = "wait_running"
	TaskStatusRunning        TaskStatus = "running"
	TaskStatusWaitPaused     TaskStatus = "wait_paused"
	TaskStatusPaused         TaskStatus = "paused"
	TaskStatusWaitStop       TaskStatus = "wait_stopped"
	TaskStatusSuccess        TaskStatus = "success"
	TaskStatusFailed         TaskStatus = "failed"

	// 调度异常
	TaskStatusExecptionRun   TaskStatus = "execption_run"
	TaskStatusExecptionPause TaskStatus = "execption_pause"
	TaskStatusExecptionStop  TaskStatus = "execption_stop"
)

func (ts TaskStatus) String() string {
	return string(ts)
}

func (ts TaskStatus) IsWaitStatus() bool {
	return strings.HasPrefix(ts.String(), "wait_")
}

func (ts TaskStatus) IsExecptionStatus() bool {
	return strings.HasPrefix(ts.String(), "execption_")
}

func (ts TaskStatus) IsFinalStatus() bool {
	return ts == TaskStatusSuccess || ts == TaskStatusFailed
}

func (ts TaskStatus) CanTransition(nextStatus TaskStatus) error {
	_, err := GetTaskStatusTransitionFunc(ts, nextStatus)
	return err
}

func (ts TaskStatus) PreWaitStatus() TaskStatus {
	if ts.IsFinalStatus() {
		return TaskStatusWaitStop
	}
	if ts == TaskStatusPaused {
		return TaskStatusWaitPaused
	}
	if ts == TaskStatusRunning {
		return TaskStatusWaitRunning
	}
	return ""
}

type TransitionFunc func(ctx context.Context, taskKey string) error

var transitionFuncsMap = make(map[TaskStatus]map[TaskStatus]TransitionFunc)

func RegisterTransitionFunc(from, to TaskStatus, fn TransitionFunc) {
	transitionFuncsMap[from][to] = fn
}

func GetTaskStatusTransitionFunc(
	realRunStatus, wantRunStatus TaskStatus,
) (TransitionFunc, error) {
	m, ok := transitionFuncsMap[realRunStatus]
	if !ok {
		return nil, errors.Errorf("当前状态为[%s], 不支持转换", realRunStatus)
	}
	fid, ok := m[wantRunStatus]
	if !ok {
		return nil, errors.Errorf("当前状态[%s] -> 期望运行状态[%s], 不支持转换", realRunStatus, wantRunStatus)
	}

	return fid, nil
}
