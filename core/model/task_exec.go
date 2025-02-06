package model

// Task Exec Param.
type TaskExecParam struct {
	TaskKey string `json:"task_key,omitempty"`
	BizID   string `json:"biz_id,omitempty"`
	BizType string `json:"biz_type,omitempty"`
	Payload string `json:"payload,omitempty"`
}

// Task Exec Result.
type TaskExecResult struct {
	TaskKey  string     `json:"task_key,omitempty"`
	TaskType string     `json:"task_type,omitempty"`
	Status   TaskStatus `json:"status,omitempty"`
	Msg      string     `json:"msg,omitempty"`
}
