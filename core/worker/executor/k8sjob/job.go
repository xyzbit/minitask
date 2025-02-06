package k8sjob

import (
	"sync"

	"github.com/xyzbit/minitaskx/core/model"
	"k8s.io/client-go/kubernetes"
	// "context"
	// "fmt"
	// "os"
	// "sync"
	// "github.com/bytedance/sonic"
	// batchv1 "k8s.io/api/batch/v1"
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/rest"
	// "k8s.io/client-go/tools/clientcmd"
	// "github.com/xyzbit/minitaskx/core/components/log"
	// "github.com/xyzbit/minitaskx/core/model"
	// "github.com/xyzbit/minitaskx/core/worker/executor"
	// "github.com/xyzbit/minitaskx/pkg/util"
)

type taskCtrl struct {
	jobName string
	task    *model.Task
}

type Executor struct {
	cli        *kubernetes.Clientset
	namespace  string
	taskrw     sync.RWMutex
	tasks      map[string]*taskCtrl
	resultChan chan *model.Task
}

// func NewExecutor() executor.Interface {
// 	var config *rest.Config
// 	var err error

// 	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
// 		config, err = rest.InClusterConfig()
// 	} else { // local dev
// 		devConfigPath := os.Getenv("KUBECONFIG")
// 		if devConfigPath == "" {
// 			devConfigPath = os.Getenv("HOME") + "/.kube/config"
// 		}
// 		config, err = clientcmd.BuildConfigFromFlags("", devConfigPath)
// 	}

// 	clientset, err := kubernetes.NewForConfig(config)
// 	if err != nil {
// 		log.Error("创建Kubernetes客户端失败: %v", err)
// 		return nil
// 	}

// 	return &Executor{
// 		cli:        clientset,
// 		namespace:  "minitaskx",
// 		tasks:      make(map[string]*taskCtrl),
// 		resultChan: make(chan *model.Task, 10),
// 	}
// }

// func (e *Executor) Run(task *model.Task) error {
// 	if task.Payload == "" {
// 		return fmt.Errorf("task payload is nil")
// 	}

// 	var config corev1.Container
// 	if err := sonic.UnmarshalString(task.Payload, &config); err != nil {
// 		return fmt.Errorf("解析容器配置失败: %v", err)
// 	}

// 	job := &batchv1.Job{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      task.TaskKey,
// 			Namespace: e.namespace,
// 		},
// 		Spec: batchv1.JobSpec{
// 			TTLSecondsAfterFinished: util.Pointer(int32(100)),
// 			BackoffLimit:            util.Pointer(int32(3)),
// 			Template: corev1.PodTemplateSpec{
// 				Spec: corev1.PodSpec{
// 					Containers:    []corev1.Container{config},
// 					RestartPolicy: corev1.RestartPolicyNever,
// 				},
// 			},
// 		},
// 	}

// 	createdJob, err := e.cli.BatchV1().Jobs(e.namespace).Create(context.Background(), job, metav1.CreateOptions{})
// 	if err != nil {
// 		return fmt.Errorf("创建Job失败: %v", err)
// 	}

// 	e.taskrw.Lock()
// 	e.tasks[task.TaskKey] = &taskCtrl{
// 		jobName: createdJob.Name,
// 		task:    task,
// 	}
// 	e.taskrw.Unlock()

// 	go e.monitorJob(task.TaskKey)

// 	task.Status = model.TaskStatusRunning
// 	e.resultChan <- task
// 	return nil
// }

// func (e *Executor) Exit(taskKey string) error {
// 	return e.stopAndRemove(taskKey, true, model.TaskStatusFailed, "force exit")
// }

// func (e *Executor) Stop(taskKey string) error {
// 	return e.stopAndRemove(taskKey, false, model.TaskStatusStop, "")
// }

// func (e *Executor) stopAndRemove(taskKey string, force bool, status model.TaskStatus, msg string) error {
// 	ctrl, err := e.getTaskCtrl(taskKey)
// 	if err != nil {
// 		return err
// 	}

// 	deletePolicy := metav1.DeletePropagationBackground
// 	if force {
// 		deletePolicy = metav1.DeletePropagationForeground
// 	}

// 	if err := e.cli.BatchV1().Jobs(e.namespace).Delete(context.Background(), ctrl.jobName, metav1.DeleteOptions{
// 		PropagationPolicy: &deletePolicy,
// 	}); err != nil {
// 		return fmt.Errorf("删除Job失败: %v", err)
// 	}

// 	e.taskrw.Lock()
// 	delete(e.tasks, taskKey)
// 	e.taskrw.Unlock()

// 	ctrl.task.Status = status
// 	ctrl.task.Msg = msg
// 	e.resultChan <- ctrl.task
// 	return nil
// }

// func (e *Executor) Pause(taskKey string) error {
// 	return fmt.Errorf("Kaniko执行器不支持暂停操作")
// }

// func (e *Executor) Resume(taskKey string) error {
// 	return fmt.Errorf("Kaniko执行器不支持恢复操作")
// }

// func (e *Executor) List(ctx context.Context) ([]*model.Task, error) {
// 	e.taskrw.RLock()
// 	defer e.taskrw.RUnlock()

// 	tasks := make([]*model.Task, 0, len(e.tasks))
// 	for _, ctrl := range e.tasks {
// 		tasks = append(tasks, ctrl.task)
// 	}
// 	return tasks, nil
// }

// func (e *Executor) ChangeResult() <-chan *model.Task {
// 	return e.resultChan
// }

// func (e *Executor) monitorJob(taskKey string) {
// 	ctrl, err := e.getTaskCtrl(taskKey)
// 	if err != nil {
// 		return
// 	}

// 	watcher, err := e.cli.BatchV1().Jobs(e.namespace).Watch(context.Background(), metav1.ListOptions{
// 		FieldSelector: fmt.Sprintf("metadata.name=%s", ctrl.jobName),
// 	})
// 	if err != nil {
// 		ctrl.task.Status = model.TaskStatusFailed
// 		ctrl.task.Msg = fmt.Sprintf("监控Job失败: %v", err)
// 		e.resultChan <- ctrl.task
// 		return
// 	}
// 	defer watcher.Stop()

// 	for event := range watcher.ResultChan() {
// 		job, ok := event.Object.(*batchv1.Job)
// 		if !ok {
// 			continue
// 		}

// 		if job.Status.Succeeded > 0 {
// 			ctrl.task.Status = model.TaskStatusSuccess
// 			break
// 		} else if job.Status.Failed > 0 {
// 			ctrl.task.Status = model.TaskStatusFailed
// 			ctrl.task.Msg = "Job执行失败"
// 			break
// 		}
// 	}

// 	e.taskrw.Lock()
// 	delete(e.tasks, taskKey)
// 	e.taskrw.Unlock()

// 	e.resultChan <- ctrl.task
// }

// func (e *Executor) getTaskCtrl(taskKey string) (*taskCtrl, error) {
// 	e.taskrw.RLock()
// 	ctrl, exists := e.tasks[taskKey]
// 	e.taskrw.RUnlock()

// 	if !exists {
// 		return nil, fmt.Errorf("task %s not found", taskKey)
// 	}
// 	return ctrl, nil
// }
