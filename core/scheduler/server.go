package scheduler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/xyzbit/minitaskx/core/log"
	"github.com/xyzbit/minitaskx/core/model"
)

type HttpServer struct {
	scheduler *Scheduler
}

func (s *HttpServer) CreateTask(c *gin.Context) {
	var req struct {
		BizID   string `json:"biz_id"`
		BizType string `json:"biz_type"`
		Type    string `json:"type"`
		Payload string `json:"payload"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Info("assign task: %+v", req)
	if req.Type == "" || req.Payload == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid params"})
		return
	}

	if err := s.scheduler.CreateTask(c.Request.Context(), &model.Task{
		BizID:   req.BizID,
		BizType: req.BizType,
		Type:    req.Type,
		Payload: req.Payload,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务分配成功"})
}

// ListTask 查询任务列表
func (s *HttpServer) ListTask(c *gin.Context) {
	var req struct {
		BizIDs  string `json:"biz_ids" form:"biz_ids"` // a,b,c
		BizType string `json:"biz_type" form:"biz_type"`
		Type    string `json:"type" form:"type"`
		Limit   int    `json:"limit" form:"limit"`   // default 20
		Offset  int    `json:"offset" form:"offset"` // default 0
	}
	if err := c.BindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit == 0 {
		req.Limit = 20
	}
	tasks, err := s.scheduler.ListTask(c.Request.Context(), &model.TaskFilter{
		BizIDs:  strings.Split(req.BizIDs, ","),
		BizType: req.BizType,
		Type:    req.Type,
		Limit:   req.Limit,
		Offset:  req.Offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

func (s *HttpServer) OperateTask(c *gin.Context) {
	var req struct {
		TaskKey string `json:"task_key"`
		Status  string `json:"status"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ts := model.TaskStatus(req.Status)
	if !lo.Contains(
		[]model.TaskStatus{
			model.TaskStatusPaused,
			model.TaskStatusRunning,
			model.TaskStatusSuccess,
			model.TaskStatusFailed,
		}, ts) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	if err := s.scheduler.OperateTask(c.Request.Context(), req.TaskKey, ts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务操作成功"})
}
