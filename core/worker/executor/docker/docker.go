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
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/worker/executor"
)

type Executor struct {
	cli *client.Client

	taskStatusrw    sync.RWMutex
	taskStatus      map[string]model.TaskStatus
	taskContainerrw sync.RWMutex
	taskContainerID map[string]string

	resultChan chan *model.TaskExecResult
}

// NewExecutor 注意设置 DOCKER_API_VERSION, 保障客户端版本兼容.
func NewExecutor() executor.Interface {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Panic("创建Docker客户端失败: %v", err)
		return nil
	}
	e := &Executor{
		cli:             cli,
		taskStatus:      make(map[string]model.TaskStatus, 0),
		taskContainerID: make(map[string]string),
		resultChan:      make(chan *model.TaskExecResult, 10),
	}

	// 恢复容器状态
	originStatus, err := e.listByOrigin(ctx)
	if err != nil {
		log.Panic("初始化容器数据是失败: %v", err)
		return nil
	}
	if len(originStatus) > 0 {
		for _, o := range originStatus {
			e.setContainerID(o.TaskKey, o.ContainerID)
			e.setTaskStatus(o.TaskKey, o.Status)
			if o.Status.IsFinalStatus() {
				continue
			}
			go e.monitorContainer(o.TaskKey, o.ContainerID)
		}
	}

	// 定时清理过期资源
	go e.cleanupResources(ctx, 5*time.Minute)
	return e
}

func (e *Executor) Run(param *model.TaskExecParam) error {
	if param.Payload == "" {
		return fmt.Errorf("task payload is nil")
	}
	ctx := context.Background()

	var config container.Config
	if err := sonic.UnmarshalString(param.Payload, &config); err != nil {
		return fmt.Errorf("解析容器配置失败: %v", err)
	}
	if config.Labels == nil {
		config.Labels = map[string]string{
			"created_by_minitaskx": "true",
		}
	} else {
		config.Labels["created_by_minitaskx"] = "true"
	}

	if err := e.pullImage(ctx, config.Image); err != nil {
		return err
	}

	resp, err := e.cli.ContainerCreate(ctx, &config, nil, nil, nil, param.TaskKey)
	if err != nil {
		return fmt.Errorf("创建容器失败: %v", err)
	}

	if err := e.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("启动容器失败: %v", err)
	}

	e.setTaskStatus(param.TaskKey, model.TaskStatusRunning)
	e.setContainerID(param.TaskKey, resp.ID)

	go e.monitorContainer(param.TaskKey, resp.ID)

	e.resultChan <- &model.TaskExecResult{
		TaskKey: param.TaskKey,
		Status:  model.TaskStatusRunning,
	}
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
	return e.stopContainer(taskKey)
}

func (e *Executor) Stop(taskKey string) error {
	return e.stopContainer(taskKey)
}

func (e *Executor) stopContainer(taskKey string) error {
	containerID, err := e.getContainerID(taskKey)
	if err != nil {
		return err
	}

	timeout := 0
	if err := e.cli.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("停止容器失败: %v", err)
	}

	return nil
}

func (e *Executor) Pause(taskKey string) error {
	containerID, err := e.getContainerID(taskKey)
	if err != nil {
		return err
	}

	if err := e.cli.ContainerPause(context.Background(), containerID); err != nil {
		return fmt.Errorf("暂停容器失败: %v", err)
	}

	e.setTaskStatus(taskKey, model.TaskStatusPaused)
	e.resultChan <- &model.TaskExecResult{
		TaskKey: taskKey,
		Status:  model.TaskStatusPaused,
	}

	return nil
}

func (e *Executor) Resume(taskKey string) error {
	containerID, err := e.getContainerID(taskKey)
	if err != nil {
		return err
	}

	if err := e.cli.ContainerUnpause(context.Background(), containerID); err != nil {
		return fmt.Errorf("恢复容器失败: %v", err)
	}

	e.setTaskStatus(taskKey, model.TaskStatusRunning)
	e.resultChan <- &model.TaskExecResult{
		TaskKey: taskKey,
		Status:  model.TaskStatusRunning,
	}
	return nil
}

func (e *Executor) List(ctx context.Context) ([]*model.TaskExecResult, error) {
	return e.listTaskStatus(), nil
}

func (e *Executor) ChangeResult() <-chan *model.TaskExecResult {
	return e.resultChan
}

func (e *Executor) monitorContainer(taskKey, containerID string) {
	statusCh, errCh := e.cli.ContainerWait(context.Background(), containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			e.setTaskStatus(taskKey, model.TaskStatusFailed)
			e.resultChan <- &model.TaskExecResult{
				TaskKey: taskKey,
				Status:  model.TaskStatusFailed,
				Msg:     err.Error(),
			}
		}
	case status := <-statusCh:
		result := &model.TaskExecResult{
			TaskKey: taskKey,
		}
		log.Info("watch container status changed, name: %s, status: %d", taskKey, status.StatusCode)

		// only monitor finshed status.
		switch status.StatusCode {
		case 0:
			result.Status = model.TaskStatusSuccess
		case 137:
			result.Status = model.TaskStatusStop
		default:
			result.Status = model.TaskStatusFailed
			result.Msg = fmt.Sprintf("容器退出码: %d", status.StatusCode)
		}

		e.setTaskStatus(taskKey, result.Status)
		e.resultChan <- result
	}
}

type containerStatus struct {
	TaskKey     string
	ContainerID string
	Status      model.TaskStatus
}

func (e *Executor) listByOrigin(ctx context.Context) ([]*containerStatus, error) {
	args := filters.NewArgs()
	args.Add("label", "created_by_minitaskx=true")
	containers, err := e.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("获取容器列表失败: %v", err)
	}

	results := make([]*containerStatus, 0, len(containers))
	for _, c := range containers {
		status := model.TaskStatusRunning
		if c.State == "exited" {
			if c.Status == "exited (0)" {
				status = model.TaskStatusSuccess
			} else {
				status = model.TaskStatusFailed
			}
		} else if c.State == "paused" {
			status = model.TaskStatusPaused
		}

		results = append(results, &containerStatus{
			TaskKey:     c.Names[0],
			ContainerID: c.ID,
			Status:      status,
		})
	}
	return results, nil
}

func (e *Executor) cleanupResources(ctx context.Context, tickerDuration time.Duration) {
	tigger := time.NewTicker(tickerDuration)
	for range tigger.C {
		tasks := e.listTaskStatus()

		for _, task := range tasks {
			if !task.Status.IsFinalStatus() {
				continue
			}
			taskKey := task.TaskKey

			containerID, err := e.getContainerID(taskKey)
			if err != nil {
				log.Error("cleanupResources, get container id error: %v", err)
				continue
			}
			if err := e.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: false}); err != nil {
				log.Error("cleanupResources, ContainerRemove failed: %v", err)
				continue
			}
			log.Debug("[Executor] cleanupResources ContainerName: %s", taskKey)

			e.taskStatusrw.Lock()
			delete(e.taskStatus, taskKey)
			e.taskStatusrw.Unlock()

			e.taskContainerrw.Lock()
			delete(e.taskContainerID, taskKey)
			e.taskContainerrw.Unlock()
		}
	}
}

func (e *Executor) setTaskStatus(taskKey string, status model.TaskStatus) {
	e.taskStatusrw.Lock()
	e.taskStatus[taskKey] = status
	e.taskStatusrw.Unlock()
}

func (e *Executor) listTaskStatus() []*model.TaskExecResult {
	results := make([]*model.TaskExecResult, 0, len(e.taskStatus))
	e.taskStatusrw.RLock()
	for taskKey, status := range e.taskStatus {
		results = append(results, &model.TaskExecResult{
			TaskKey: taskKey,
			Status:  status,
		})
	}
	e.taskStatusrw.RUnlock()
	return results
}

func (e *Executor) setContainerID(taskKey, containerID string) {
	e.taskContainerrw.Lock()
	e.taskContainerID[taskKey] = containerID
	e.taskContainerrw.Unlock()
}

func (e *Executor) getContainerID(taskKey string) (string, error) {
	e.taskContainerrw.RLock()
	containerID, exists := e.taskContainerID[taskKey]
	e.taskContainerrw.RUnlock()
	if !exists {
		return "", fmt.Errorf("task %s not found", taskKey)
	}
	return containerID, nil
}
