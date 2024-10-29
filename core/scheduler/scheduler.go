package scheduler

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/xyzbit/minitaskx/core/discover"
	"github.com/xyzbit/minitaskx/core/election"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/taskrepo"
	"github.com/xyzbit/minitaskx/pkg/log"
	"golang.org/x/exp/rand"
)

var random = rand.New(rand.NewSource(uint64(time.Now().UnixNano())))

type Scheduler struct {
	rwmu sync.RWMutex

	availableWorkers atomic.Value
	assignEvent      chan struct{}

	discover discover.Interface
	elector  election.Interface
	taskRepo taskrepo.Interface

	logger log.Logger
}

func NewScheduler(
	elector election.Interface,
	discover discover.Interface,
	taskRepo taskrepo.Interface,
) (*Scheduler, error) {
	return &Scheduler{
		elector:  elector,
		discover: discover,
		taskRepo: taskRepo,
	}, nil
}

func (s *Scheduler) HttpServer() *HttpServer {
	return &HttpServer{scheduler: s}
}

func (s *Scheduler) Run() error {
	availableWorkers, err := s.discover.GetAvailableInstances()
	if err != nil {
		return fmt.Errorf("获取 worker 服务列表失败: %v", err)
	}
	s.setAvailableWorkers(availableWorkers)
	s.assignEvent = make(chan struct{}, 1)

	go s.elector.AttemptElection()
	go s.monitorAssignEvent()
	go s.autoTriggerReAssignEvent()

	return s.watchWorkers()
}

func (s *Scheduler) CreateTask(ctx context.Context, task *model.Task) error {
	// _, exist := executor.GetFactory(task.Type)
	// if !exist {
	// 	return errors.Errorf("任务类型 %s 不存在", task.Type)
	// }
	return s.createTask(ctx, task)
}

func (s *Scheduler) OperateTask(ctx context.Context, taskKey string, nextStatus model.TaskStatus) error {
	task, err := s.taskRepo.GetTask(ctx, taskKey)
	if err != nil {
		return err
	}
	// if err := task.Status.CanTransition(nextStatus); err != nil {
	// 	return err
	// }

	waitStatus := nextStatus.PreWaitStatus()
	if waitStatus == "" {
		return errors.Errorf("任务[%s]当前状态为 %s, 不允许进行 %s 操作", task.TaskKey, task.Status, nextStatus)
	}

	err = s.taskRepo.UpdateTaskTX(
		ctx,
		&model.Task{
			TaskKey: task.TaskKey,
			Status:  waitStatus,
		},
		&model.TaskRun{
			TaskKey:       taskKey,
			WantRunStatus: nextStatus,
		},
	)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Scheduler) createTask(ctx context.Context, task *model.Task) error {
	task.TaskKey = uuid.New().String()
	task.Status = model.TaskStatusWaitScheduling
	taskRun := &model.TaskRun{TaskKey: task.TaskKey}

	err := s.taskRepo.CreateTaskTX(ctx, task, taskRun)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Scheduler) assignTask(ctx context.Context, task *model.Task) error {
	if task.Status == model.TaskStatusWaitScheduling {
		log.Info("任务[%s]首次分配工作者", task.TaskKey)
	} else {
		log.Info("任务[%s]需要重新分配, 工作者替换", task.TaskKey)
	}
	workerID, err := s.selectWorkerID(task)
	if err != nil {
		return err
	}

	nextStatus := model.TaskStatusRunning
	now := time.Now()
	err = s.taskRepo.UpdateTaskTX(
		ctx,
		&model.Task{
			TaskKey: task.TaskKey,
			Status:  nextStatus.PreWaitStatus(),
		},
		&model.TaskRun{
			TaskKey:       task.TaskKey,
			NextRunAt:     &now,
			WorkerID:      workerID,
			WantRunStatus: nextStatus,
		},
	)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Scheduler) monitorAssignEvent() {
	for range s.assignEvent {
		amILeader, _, err := s.amILeader()
		if err != nil {
			log.Error("获取 leader 状态失败: %v", err)
			continue
		}
		if !amILeader {
			log.Error("当前节点不是 leader, 不进行任务重新分配")
			continue
		}

		ctx := context.Background()
		tasks, err := s.loadNeedAssignTasks(ctx)
		if err != nil {
			log.Error("获取任务列表失败: %+v", err)
			continue
		}

		for _, task := range tasks {
			if err := s.assignTask(ctx, task); err != nil {
				log.Error("任务[%s]分配失败, err: %v", task.TaskKey, err)
			}
		}
	}
}

func (s *Scheduler) loadNeedAssignTasks(ctx context.Context) ([]*model.Task, error) {
	newAvailableWorkers := s.getAvailableWorkers()
	taskRuns, err := s.taskRepo.ListTaskRuns(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	taskKeys := make([]string, 0, len(taskRuns))
	for _, run := range taskRuns {
		found := slices.ContainsFunc(newAvailableWorkers, func(newWorker discover.Instance) bool {
			return newWorker.ID() == run.WorkerID
		})
		if !found {
			taskKeys = append(taskKeys, run.TaskKey)
		}
	}

	tasks, err := s.taskRepo.BatchGetTask(ctx, taskKeys)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return tasks, nil
}

func (s *Scheduler) amILeader() (bool, *election.LeaderElection, error) {
	leader, err := s.elector.Leader()
	if err != nil {
		return false, leader, fmt.Errorf("获取 leader 状态失败: %v", err)
	}
	return s.elector.AmILeader(leader), leader, nil
}

func (s *Scheduler) watchWorkers() error {
	return s.discover.Subscribe(
		func(services []discover.Instance, err error) {
			newAvailableWorkers := make([]discover.Instance, 0)
			for _, newWorker := range services {
				if newWorker.Healthy {
					newAvailableWorkers = append(newAvailableWorkers, newWorker)
				}
			}

			if s.hasAvailableWorkersChanged(newAvailableWorkers) {
				log.Info("可用 worker 发生变化, 部分任务需要重新分配")
				// 加锁更新 availableWorkers，保证开始 rebalance 时不会再往错误节点分配任务.
				s.rwmu.Lock()
				s.setAvailableWorkers(newAvailableWorkers)
				s.rwmu.Unlock()

				s.triggerReAssignEvent()
			} else {
				s.setAvailableWorkers(newAvailableWorkers)
			}
		},
	)
}

type workerScore struct {
	index int
	score float64
}

func (s *Scheduler) selectWorkerID(task *model.Task) (string, error) {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()

	availableWorkers := s.getAvailableWorkers()
	if len(availableWorkers) == 0 {
		return "", errors.New("没有可用的 worker 服务")
	}

	// filte 排除掉不部署的机器（污点、亲和性）
	candidateWorkers := filteWorker(task, availableWorkers)
	if len(candidateWorkers) == 0 {
		return "", errors.New("没有可用的 worker")
	}
	if len(candidateWorkers) == 1 {
		return candidateWorkers[0].InstanceId, nil
	}

	// priority 根据资源使用情况打分
	selectedWorker := priorityWorker(candidateWorkers)

	s.updateLocalResourceEstimate(selectedWorker)

	log.Info("选择 worker InstanceId: %s", selectedWorker.InstanceId)

	return selectedWorker.InstanceId, nil
}

func (s *Scheduler) hasAvailableWorkersChanged(newAvailableWorkers []discover.Instance) bool {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()

	if len(s.getAvailableWorkers()) != len(newAvailableWorkers) {
		return true
	}

	for _, newWorker := range newAvailableWorkers {
		found := false
		for _, oldWorker := range s.getAvailableWorkers() {
			if newWorker.ID() == oldWorker.ID() {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}

	return false
}

func (s *Scheduler) autoTriggerReAssignEvent() {
	for {
		s.triggerReAssignEvent()
		time.Sleep(1*time.Second + time.Duration(rand.Intn(1000))*time.Millisecond)
	}
}

func (s *Scheduler) triggerReAssignEvent() {
	s.assignEvent <- struct{}{}
}

func (s *Scheduler) setAvailableWorkers(instances []discover.Instance) {
	s.availableWorkers.Store(instances)
}

func (s *Scheduler) getAvailableWorkers() []discover.Instance {
	workers := s.availableWorkers.Load().([]discover.Instance)
	newWorkers := make([]discover.Instance, len(workers))
	copy(newWorkers, workers)
	return newWorkers
}

func (s *Scheduler) updateLocalResourceEstimate(worker discover.Instance) {
	resourceUsage := model.ParseResourceUsage(worker.Metadata)
	cpuUsage := resourceUsage[model.CpuUsageKey]
	memUsage := resourceUsage[model.MemUsageKey]
	goroutineNum := resourceUsage[model.GoGoroutineKey]

	worker.Metadata[model.CpuUsageKey] = strconv.FormatFloat(cpuUsage+0.05, 'f', 2, 64)
	worker.Metadata[model.MemUsageKey] = strconv.FormatFloat(memUsage+0.05, 'f', 2, 64)
	worker.Metadata[model.GoGoroutineKey] = strconv.FormatFloat(goroutineNum+1, 'f', 2, 64)

	s.updateWorkerInCache(worker)
}

func (s *Scheduler) updateWorkerInCache(updatedWorker discover.Instance) {
	workers := s.getAvailableWorkers()
	for i, worker := range workers {
		if worker.InstanceId == updatedWorker.InstanceId {
			workers[i] = updatedWorker
			break
		}
	}
	s.setAvailableWorkers(workers)
}

func filteWorker(task *model.Task, workers []discover.Instance) []discover.Instance {
	candidateWorkers := make([]discover.Instance, 0, len(workers))

	for _, worker := range workers {
		nodeStaints := model.ParseStaint(worker.Metadata)
		if len(nodeStaints) == 0 {
			candidateWorkers = append(candidateWorkers, worker)
			continue
		}
		if len(nodeStaints) > len(task.Staints) {
			continue
		}

		matched := true
		for k, nodev := range nodeStaints {
			if task.Staints[k] != nodev {
				matched = false
				break
			}
		}
		if matched {
			candidateWorkers = append(candidateWorkers, worker)
		}
	}
	return candidateWorkers
}

func priorityWorker(workers []discover.Instance) discover.Instance {
	scores := make([]workerScore, 0, len(workers))
	for i, worker := range workers {
		resourceUsage := model.ParseResourceUsage(worker.Metadata)
		cpuScore := resourceUsage[model.CpuUsageKey]
		memoryScore := resourceUsage[model.MemUsageKey]
		goroutineNum := resourceUsage[model.GoGoroutineKey]
		gcPause := resourceUsage[model.GoGcPauseKey]
		gcCount := resourceUsage[model.GoGcCountKey]

		score := cpuScore*0.5 + memoryScore*0.5

		if gcCount == 0 {
			score += float64(goroutineNum)
		} else {
			// 最大的 goroutine 数和 gc 平均耗时微秒(通常情况每次10-30微秒, stw 可能达到 10-50ms, 正常情况平均 10-500 微秒间)
			maxGoroutineNum, maxGcMicrosecond := float64(5000), float64(500)
			goroutineScore := (float64(goroutineNum) / maxGoroutineNum) * 100
			gcScore := (float64(gcPause) / float64(gcCount)) / maxGcMicrosecond * 100
			score += goroutineScore*0.5 + gcScore*0.5
		}

		scores = append(scores, workerScore{
			index: i,
			score: score,
		})
	}
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})

	log.Info("worker scores: %v", scores)
	return workers[scores[0].index]
}
