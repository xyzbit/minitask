package model

import (
	"github.com/pkg/errors"
	"github.com/xyzbit/minitaskx/internal/queue"
)

var _ queue.UniKey[Change] = Change{}

type ChangeType string

const (
	ChangeCreate ChangeType = "create"
	ChangeDelete ChangeType = "delete"
	ChangeResume ChangeType = "resume"
	ChangePause  ChangeType = "pause"
	ChangeStop   ChangeType = "stop"
)

var changeTypesRule = map[TaskStatus]map[TaskStatus]ChangeType{
	TaskStatusNotExist: {
		TaskStatusRunning: ChangeCreate,
	},
	TaskStatusRunning: {
		TaskStatusPaused:   ChangePause,
		TaskStatusStop:     ChangeStop,
		TaskStatusNotExist: ChangeDelete,
	},
	TaskStatusPaused: {
		TaskStatusRunning:       ChangeResume,
		TaskStatusExecptionStop: ChangeStop,
		TaskStatusNotExist:      ChangeDelete,
	},
}

func GetChangeType(
	realRunStatus, wantRunStatus TaskStatus,
) (ChangeType, error) {
	m, ok := changeTypesRule[realRunStatus]
	if !ok {
		return "", errors.Errorf("当前状态为[%s], 不支持转换", realRunStatus)
	}
	fid, ok := m[wantRunStatus]
	if !ok {
		return "", errors.Errorf("当前状态[%s] -> 期望运行状态[%s], 不支持转换", realRunStatus, wantRunStatus)
	}

	return fid, nil
}

type Change struct {
	TaskKey    string
	TaskType   string
	ChangeType ChangeType
	Task       *Task
}

func (c Change) GetUniKey() Change {
	return Change{TaskKey: c.TaskKey}
}
