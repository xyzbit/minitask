package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/worker/executor"
)

type taskCtrl struct {
	containerID string
	task        *model.Task
}

type Executor struct {
	cli        *client.Client
	taskrw     sync.RWMutex
	tasks      map[string]*taskCtrl
	resultChan chan *model.Task
}

// NewExecutor 注意设置 DOCKER_API_VERSION, 保障客户端版本兼容.
func NewExecutor() executor.Interface {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Error("创建Docker客户端失败: %v", err)
		return nil
	}
	return &Executor{
		cli:        cli,
		tasks:      make(map[string]*taskCtrl),
		resultChan: make(chan *model.Task, 10),
	}
}

func (e *Executor) Run(task *model.Task) error {
	if task.Payload == "" {
		return fmt.Errorf("task payload is nil")
	}
	ctx := context.Background()

	var config container.Config
	if err := sonic.UnmarshalString(task.Payload, &config); err != nil {
		return fmt.Errorf("解析容器配置失败: %v", err)
	}

	if err := e.pullImage(ctx, config.Image); err != nil {
		return err
	}

	resp, err := e.cli.ContainerCreate(ctx, &config, nil, nil, nil, task.TaskKey)
	if err != nil {
		return fmt.Errorf("创建容器失败: %v", err)
	}

	if err := e.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("启动容器失败: %v", err)
	}

	e.taskrw.Lock()
	e.tasks[task.TaskKey] = &taskCtrl{
		containerID: resp.ID,
		task:        task,
	}
	e.taskrw.Unlock()

	go e.monitorContainer(task.TaskKey)

	task.Status = model.TaskStatusRunning
	e.resultChan <- task
	return nil
}

func (e *Executor) pullImage(ctx context.Context, imageUrl string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Try with original image URL
	reader, err := e.cli.ImagePull(ctx, imageUrl, image.PullOptions{})
	if err != nil {
		// If failed, try with mirror registry
		mirrorUrls := []string{
			"mirror.iscas.ac.cn",
			"ccr.ccs.tencentyun.com",
			"dockerproxy.com",
		}

		for _, mirror := range mirrorUrls {
			mirrorImageUrl := mirror + "/" + imageUrl
			reader, err = e.cli.ImagePull(ctx, mirrorImageUrl, image.PullOptions{})
			if err == nil {
				break
			}
		}

		if err != nil {
			return fmt.Errorf("拉取镜像失败: %v", err)
		}
	}
	defer reader.Close()

	type pullProgress struct {
		Status         string `json:"status"`
		ProgressDetail struct {
			Current int64 `json:"current"`
			Total   int64 `json:"total"`
		} `json:"progressDetail"`
	}

	decoder := json.NewDecoder(reader)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("拉取镜像超时")
		default:
			var progress pullProgress
			if err := decoder.Decode(&progress); err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("解析进度信息失败: %v", err)
			}
			log.Info("拉取镜像进度: %s (%d/%d)", progress.Status, progress.ProgressDetail.Current, progress.ProgressDetail.Total)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (e *Executor) Exit(taskKey string) error {
	return e.stopAndRemove(taskKey, true, model.TaskStatusFailed, "force exit")
}

func (e *Executor) Stop(taskKey string) error {
	return e.stopAndRemove(taskKey, false, model.TaskStatusStop, "")
}

func (e *Executor) stopAndRemove(taskKey string, force bool, status model.TaskStatus, msg string) error {
	ctrl, err := e.getTaskCtrl(taskKey)
	if err != nil {
		return err
	}

	timeout := 0
	if err := e.cli.ContainerStop(context.Background(), ctrl.containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("停止容器失败: %v", err)
	}

	if err := e.cli.ContainerRemove(context.Background(), ctrl.containerID, container.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("删除容器失败: %v", err)
	}

	e.taskrw.Lock()
	delete(e.tasks, taskKey)
	e.taskrw.Unlock()

	ctrl.task.Status = status
	e.resultChan <- ctrl.task
	return nil
}

func (e *Executor) Pause(taskKey string) error {
	ctrl, err := e.getTaskCtrl(taskKey)
	if err != nil {
		return err
	}

	if err := e.cli.ContainerPause(context.Background(), ctrl.containerID); err != nil {
		return fmt.Errorf("暂停容器失败: %v", err)
	}

	ctrl.task.Status = model.TaskStatusPaused
	e.resultChan <- ctrl.task
	return nil
}

func (e *Executor) Resume(taskKey string) error {
	ctrl, err := e.getTaskCtrl(taskKey)
	if err != nil {
		return err
	}

	if err := e.cli.ContainerUnpause(context.Background(), ctrl.containerID); err != nil {
		return fmt.Errorf("恢复容器失败: %v", err)
	}

	ctrl.task.Status = model.TaskStatusRunning
	e.resultChan <- ctrl.task
	return nil
}

func (e *Executor) List(ctx context.Context) ([]*model.Task, error) {
	e.taskrw.RLock()
	defer e.taskrw.RUnlock()

	tasks := make([]*model.Task, 0, len(e.tasks))
	for _, ctrl := range e.tasks {
		tasks = append(tasks, ctrl.task)
	}
	return tasks, nil
}

func (e *Executor) ChangeResult() <-chan *model.Task {
	return e.resultChan
}

func (e *Executor) monitorContainer(taskKey string) {
	ctrl, err := e.getTaskCtrl(taskKey)
	if err != nil {
		return
	}

	statusCh, errCh := e.cli.ContainerWait(context.Background(), ctrl.containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			ctrl.task.Status = model.TaskStatusFailed
			ctrl.task.Msg = err.Error()
		}
	case status := <-statusCh:
		if status.StatusCode == 0 {
			ctrl.task.Status = model.TaskStatusSuccess
		} else {
			ctrl.task.Status = model.TaskStatusFailed
			ctrl.task.Msg = fmt.Sprintf("容器退出码: %d", status.StatusCode)
		}
	}

	e.taskrw.Lock()
	delete(e.tasks, taskKey)
	e.taskrw.Unlock()

	e.resultChan <- ctrl.task
}

func (e *Executor) getTaskCtrl(taskKey string) (*taskCtrl, error) {
	e.taskrw.RLock()
	ctrl, exists := e.tasks[taskKey]
	e.taskrw.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task %s not found", taskKey)
	}
	return ctrl, nil
}
