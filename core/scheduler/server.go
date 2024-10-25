package scheduler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
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
	log.Printf("assign task: %+v", req)

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

func (s *HttpServer) OperateTask(c *gin.Context) {
	var req struct {
		TaskKey string `json:"task_key"`
		Status  string `json:"status"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.scheduler.OperateTask(c.Request.Context(), req.TaskKey, model.TaskStatus(req.Status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务操作成功"})
}
