package model

type TaskResult struct {
	Status TaskStatus `json:"status,omitempty"`
	Msg    string     `json:"msg,omitempty"`
}
