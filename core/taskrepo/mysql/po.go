package mysql

import "time"

type Task struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TaskKey   string    `gorm:"column:task_key;not null;comment:任务唯一标识"`
	BizID     string    `gorm:"column:biz_id"`
	BizType   string    `gorm:"column:biz_type"`
	Type      string    `gorm:"column:type;not null;comment:任务类型"`
	Payload   string    `gorm:"column:payload;not null;comment:任务内容"`
	Tags      *string   `gorm:"column:tags;type:json;comment:任务标签"`
	Extra     *string   `gorm:"column:extra"`
	Status    string    `gorm:"column:status;not null;comment:pending scheduled running|puase success failed"`
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
