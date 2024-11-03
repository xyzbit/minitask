package mysql

import (
	"encoding/json"
	"time"

	"github.com/xyzbit/minitaskx/core/model"
)

type Task struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TaskKey   string    `gorm:"column:task_key;not null;comment:任务唯一标识"`
	BizID     string    `gorm:"column:biz_id"`
	BizType   string    `gorm:"column:biz_type"`
	Type      string    `gorm:"column:type;not null;comment:任务类型"`
	Payload   string    `gorm:"column:payload;not null;comment:任务内容"`
	Labels    *string   `gorm:"column:labels;type:json;comment:任务标签"`
	Staints   *string   `gorm:"column:staints;type:json;comment:任务污点"`
	Extra     *string   `gorm:"column:extra"`
	Status    string    `gorm:"column:status;not null;comment:pending scheduled running|puase success failed"`
	Result    *string   `gorm:"column:result;type:json;comment:任务结果"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Task) TableName() string {
	return "task"
}

type TaskRun struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TaskKey       string     `gorm:"column:task_key;not null;comment:任务唯一标识"`
	WorkerID      string     `gorm:"column:worker_id;not null;comment:工作者id"`
	NextRunAt     *time.Time `gorm:"column:next_run_at;comment:下一次执行时间"`
	WantRunStatus string     `gorm:"column:want_run_status;not null;comment:期望的运行状态: running puased success failed"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (TaskRun) TableName() string {
	return "task_run"
}

func FromTaskModel(t *model.Task) *Task {
	if t == nil {
		return nil
	}

	task := &Task{
		ID:        t.ID,
		TaskKey:   t.TaskKey,
		BizID:     t.BizID,
		BizType:   t.BizType,
		Type:      t.Type,
		Payload:   t.Payload,
		Status:    t.Status.String(),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.Labels != nil {
		labels, _ := json.Marshal(t.Labels)
		labelsStr := string(labels)
		task.Labels = &labelsStr
	}
	if t.Staints != nil {
		staints, _ := json.Marshal(t.Staints)
		staintsStr := string(staints)
		task.Staints = &staintsStr
	}
	if t.Extra != nil {
		extra, _ := json.Marshal(t.Extra)
		extraStr := string(extra)
		task.Extra = &extraStr
	}

	return task
}

func ToTaskModel(t *Task) *model.Task {
	if t == nil {
		return nil
	}

	task := &model.Task{
		ID:        t.ID,
		TaskKey:   t.TaskKey,
		BizID:     t.BizID,
		BizType:   t.BizType,
		Type:      t.Type,
		Payload:   t.Payload,
		Status:    model.TaskStatus(t.Status),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	if t.Labels != nil && *t.Labels != "" {
		var labels map[string]string
		_ = json.Unmarshal([]byte(*t.Labels), &labels)
		task.Labels = labels
	}
	if t.Staints != nil && *t.Labels != "" {
		var staints map[string]string
		_ = json.Unmarshal([]byte(*t.Staints), &staints)
		task.Staints = staints
	}
	if t.Extra != nil && *t.Extra != "" {
		var extra map[string]string
		_ = json.Unmarshal([]byte(*t.Extra), &extra)
		task.Extra = extra
	}

	return task
}

func FromTaskRunModel(tr *model.TaskRun) *TaskRun {
	if tr == nil {
		return nil
	}

	return &TaskRun{
		ID:            tr.ID,
		TaskKey:       tr.TaskKey,
		WorkerID:      tr.WorkerID,
		NextRunAt:     tr.NextRunAt,
		WantRunStatus: tr.WantRunStatus.String(),
		CreatedAt:     tr.CreatedAt,
		UpdatedAt:     tr.UpdatedAt,
	}
}

func ToTaskRunModel(tr *TaskRun) *model.TaskRun {
	if tr == nil {
		return nil
	}

	return &model.TaskRun{
		ID:            tr.ID,
		TaskKey:       tr.TaskKey,
		WorkerID:      tr.WorkerID,
		NextRunAt:     tr.NextRunAt,
		WantRunStatus: model.TaskStatus(tr.WantRunStatus),
		CreatedAt:     tr.CreatedAt,
		UpdatedAt:     tr.UpdatedAt,
	}
}
