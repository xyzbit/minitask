package model

import "time"

type Task struct {
	ID            int64             `json:"id,omitempty"`
	TaskKey       string            `json:"task_key,omitempty"`
	BizID         string            `json:"biz_id,omitempty"`
	BizType       string            `json:"biz_type,omitempty"`
	Type          string            `json:"type,omitempty"`
	Payload       string            `json:"payload,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Stains        map[string]string `json:"stains,omitempty"`
	Extra         map[string]string `json:"extra,omitempty"`
	Status        TaskStatus        `json:"status,omitempty"`          // current real status
	WantRunStatus TaskStatus        `json:"want_run_status,omitempty"` // want status
	WorkerID      string            `json:"worker_id,omitempty"`
	NextRunAt     *time.Time        `json:"next_run_at,omitempty"`
	Msg           string            `json:"msg,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at,omitempty"`
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
		Stains:    t.Stains,
		Extra:     t.Extra,
		Status:    t.Status,
		Msg:       t.Msg,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

type TaskFilter struct {
	BizIDs  []string
	BizType string
	Type    string

	Offset int
	Limit  int
}
