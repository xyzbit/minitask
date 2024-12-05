package model

import "time"

type Task struct {
	ID        int64
	TaskKey   string
	BizID     string
	BizType   string
	Type      string
	Payload   string
	Labels    map[string]string
	Staints   map[string]string
	Extra     map[string]string
	Status    TaskStatus
	Msg       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *Task) Clone() *Task {
	return &Task{
		ID:        t.ID,
		TaskKey:   t.TaskKey,
		BizID:     t.BizID,
		BizType:   t.BizType,
		Type:      t.Type,
		Payload:   t.Payload,
		Labels:    t.Labels,
		Staints:   t.Staints,
		Extra:     t.Extra,
		Status:    t.Status,
		Msg:       t.Msg,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

type TaskRun struct {
	ID            int64
	TaskKey       string
	WorkerID      string
	NextRunAt     *time.Time
	WantRunStatus TaskStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TaskFilter struct {
	BizIDs  []string
	BizType string
	Type    string

	Offset int
	Limit  int
}
