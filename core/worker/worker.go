package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/xyzbit/minitaskx/core/discover"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/taskrepo"
)

type Worker struct {
	id   string
	ip   string
	port int

	// 记录任务当前实际运行状态
	// task_key -> run_status
	taskStatus sync.Map
	// 任务执行器，worker 会管理它们的生命周期
	// task_key -> executor
	executors sync.Map

	discover discover.Interface
	taskRepo taskrepo.Interface
}

func NewWorker(
	id string,
	ip string,
	port int,
	discover discover.Interface,
	taskRepo taskrepo.Interface,
) *Worker {
	w := &Worker{
		id:       id,
		ip:       ip,
		port:     port,
		discover: discover,
		taskRepo: taskRepo,
	}

	w.initTransitionFuncs()
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	metadata, err := generateInstanceMetadata()
	if err != nil {
		return fmt.Errorf("生成实例元数据失败: %v", err)
	}

	success, err := w.discover.Register(discover.Instance{
		Ip:       w.ip,
		Port:     uint64(w.port),
		Enable:   true,
		Healthy:  true,
		Metadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("注册实例失败: %v", err)
	}
	if !success {
		return fmt.Errorf("注册实例失败")
	}
	defer func() {
		_, err := w.discover.UnRegister(discover.Instance{
			Ip:   w.ip,
			Port: uint64(w.port),
		})
		if err != nil {
			log.Printf("注销实例失败: %v", err)
		}
	}()

	if w.id == "" {
		err = w.setInstanceID()
		if err != nil {
			return fmt.Errorf("设置实例 ID 失败: %v", err)
		}
	}

	go w.reportResourceUsage(5 * time.Second)
	go w.syncRealStatusToTaskRecord(5 * time.Second)

	return w.run(ctx)
}

func (w *Worker) setInstanceID() error {
	// 获取当前实例的 InstanceId(注册存在延迟，重试获取)
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		instances, err := w.discover.GetAvailableInstances()
		if err != nil {
			return fmt.Errorf("获取实例列表失败: %v", err)
		}

		for _, instance := range instances {
			if instance.Ip == w.ip && instance.Port == uint64(w.port) {
				w.id = instance.InstanceId
				break
			}
		}
		if w.id != "" {
			break
		}
	}

	if w.id == "" {
		return fmt.Errorf("未找到当前实例的 InstanceId")
	}
	return nil
}

func (w *Worker) reportResourceUsage(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		metadata, err := generateInstanceMetadata()
		if err != nil {
			log.Printf("获取资源使用情况失败: %v", err)
			continue
		}

		_, err = w.nacosClient.UpdateInstance(vo.UpdateInstanceParam{
			Ip:          w.ip,
			Port:        uint64(w.port),
			ServiceName: "worker-service",
			GroupName:   "DEFAULT_GROUP",
			ClusterName: "DEFAULT",
			Weight:      1,
			Enable:      true,
			Healthy:     true,
			Metadata:    metadata,
		})
		if err != nil {
			log.Printf("更新服务实例元数据失败: %v", err)
		}
	}
}

func (w *Worker) getRealRunStatus(taskKey string) model.TaskStatus {
	realTaskStatusAny, ok := w.taskStatus.Load(taskKey)
	if !ok {
		return model.TaskStatusNotExist
	}
	realRunStatus, _ := realTaskStatusAny.(model.TaskStatus)
	return realRunStatus
}

func (w *Worker) setRealRunStatus(taskKey string, wantRunStatus model.TaskStatus) {
	if wantRunStatus == model.TaskStatusNotExist {
		w.taskStatus.Delete(taskKey)
		return
	}
	w.taskStatus.Store(taskKey, wantRunStatus)
}
