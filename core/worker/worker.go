package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/xyzbit/minitaskx/core/components/discover"
	"github.com/xyzbit/minitaskx/core/components/log"
	"github.com/xyzbit/minitaskx/core/components/taskrepo"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/worker/executor"
	"github.com/xyzbit/minitaskx/core/worker/infomer"
	"github.com/xyzbit/minitaskx/pkg/util/retry"
	"golang.org/x/exp/rand"
)

type Worker struct {
	id   string
	ip   string
	port int

	discover discover.Interface

	infomer    *infomer.Infomer
	exeManager *executor.Manager

	opts *options
}

func NewWorker(
	id string,
	ip string,
	port int,
	discover discover.Interface,
	taskRepo taskrepo.Interface,
	opts ...Option,
) *Worker {
	w := &Worker{
		id:       id,
		ip:       ip,
		port:     port,
		discover: discover,
		opts:     newOptions(opts...),
	}

	manager := &executor.Manager{}
	w.infomer = infomer.New(
		infomer.NewIndexer(manager, w.opts.resync),
		taskRepo,
		w.opts.logger,
	)
	w.exeManager = manager
	return w
}

func (w *Worker) Run(ctx context.Context) error {
	// init
	clear, err := w.init()
	if err != nil {
		return err
	}
	defer clear()

	// start run
	w.opts.logger.Info("Worker[%s] 开始运行...", w.id)

	go w.runResourceUsageReporter()
	go w.runChangeSyncer()
	go w.runInfomer(ctx)

	// wait ctx cancel
	<-ctx.Done()
	w.opts.logger.Info("Worker[%s] 开始退出...", w.id)
	return w.gracefulShutdown()
}

func (w *Worker) init() (clear func() error, err error) {
	// register instance
	metadata, err := w.generateInstanceMetadata()
	if err != nil {
		return nil, fmt.Errorf("[Worker] init, generateInstanceMetadata: %v", err)
	}
	success, err := w.discover.Register(discover.Instance{
		Ip:       w.ip,
		Port:     uint64(w.port),
		Enable:   true,
		Healthy:  true,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("[Worker] init, register instance: %v", err)
	}
	if !success {
		return nil, fmt.Errorf("[Worker] init, register instance failed")
	}

	// generate instance id if not exist
	if w.id == "" {
		if err := w.setInstanceID(); err != nil {
			return nil, fmt.Errorf("[Worker] init, setInstanceID: %v", err)
		}
	}

	return func() error {
		_, err := w.discover.UnRegister(discover.Instance{
			Ip:   w.ip,
			Port: uint64(w.port),
		})
		return err
	}, nil
}

func (w *Worker) runInfomer(ctx context.Context) {
	if err := w.infomer.Run(ctx, w.id, w.opts.resync); err != nil {
		w.opts.logger.Error("[Worker] infomer run failed: %v", err)
	}
}

func (w *Worker) runChangeSyncer() {
	consumer := w.infomer.ChangeConsumer()

	for {
		change, isShutdown := consumer.WaitChange()
		if isShutdown {
			log.Info("[Worker] consumer shutdown.")
			break
		}

		if err := w.exeManager.ChangeHandle(&change); err != nil {
			log.Error("[Worker] change sync failed: %v", err)
			consumer.JumpChange(change)
		}
	}
}

func (w *Worker) gracefulShutdown() error {
	// mark instance disable, worker will no longer be assigned tasks in the future.
	stain, _ := model.GenerateStain(map[string]string{}, true)
	err := retry.Do(func() error {
		return w.discover.UpdateInstance(discover.Instance{
			Ip:       w.ip,
			Port:     uint64(w.port),
			Enable:   false,
			Healthy:  true,
			Metadata: stain,
		})
	})
	if err != nil {
		log.Error("[Worker] gracefulShutdown mark instance disable: %v", err)
	}

	// wait infomer shutdown.
	stopCtx := context.Background()
	if w.opts.shutdownTimeout > 0 {
		var cancel context.CancelFunc
		stopCtx, cancel = context.WithTimeout(stopCtx, w.opts.shutdownTimeout)
		defer cancel()
	}

	return w.infomer.Shutdown(stopCtx)
}

func (w *Worker) setInstanceID() error {
	// 获取当前实例的 InstanceId(注册存在延迟，重试获取)
	for i := 0; i < 10; i++ {
		if w.id != "" {
			break
		}

		time.Sleep(1 * time.Second)
		instances, err := w.discover.GetAvailableInstances()
		if err != nil {
			w.opts.logger.Error("获取实例列表失败: %v", err)
			continue
		}

		for _, instance := range instances {
			if instance.Ip == w.ip && instance.Port == uint64(w.port) {
				w.id = instance.InstanceId
				break
			}
		}
	}

	if w.id == "" {
		return fmt.Errorf("未找到当前实例的 InstanceId")
	}
	return nil
}

func (w *Worker) runResourceUsageReporter() {
	for {
		metadata, err := w.generateInstanceMetadata()
		if err != nil {
			w.opts.logger.Error("获取资源使用情况失败: %v", err)
			continue
		}

		err = w.discover.UpdateInstance(discover.Instance{
			Ip:       w.ip,
			Port:     uint64(w.port),
			Enable:   true,
			Healthy:  true,
			Metadata: metadata,
		})
		if err != nil {
			w.opts.logger.Error("runResourceUsageReporter UpdateInstance: %v", err)
		}

		time.Sleep(w.opts.reportResourceInterval + time.Duration(rand.Intn(500))*time.Millisecond)
	}
}
