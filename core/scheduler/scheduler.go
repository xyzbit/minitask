package scheduler

import (
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
	"github.com/xyzbit/minitaskx/core/executor"
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

func (s *Scheduler) HttpServer(scheduler *Scheduler) *HttpServer {
	return &HttpServer{scheduler: scheduler}
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

func (s *Scheduler) CreateTask(task *model.Task) error {
	_, exist := executor.GetFactory(task.Type)
	if !exist {
		return errors.Errorf("任务类型 %s 不存在", task.Type)
	}
	return s.createTask(task)
}

func (s *Scheduler) OperateTask(taskKey string, nextStatus model.TaskStatus) error {
	task, err := s.taskRepo.GetTask(taskKey)
	if err != nil {
		return err
	}
	if err := task.Status.CanTransition(nextStatus); err != nil {
		return err
	}

	waitStatus := nextStatus.PreWaitStatus()
	if waitStatus == "" {
		return errors.Errorf("任务[%s]当前状态为 %s, 不允许进行 %s 操作", task.TaskKey, task.Status, nextStatus)
	}

	err = s.taskRepo.UpdateTaskTX(
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

func (s *Scheduler) createTask(task *model.Task) error {
	task.TaskKey = uuid.New().String()
	task.Status = model.TaskStatusWaitScheduling
	taskRun := &model.TaskRun{TaskKey: task.TaskKey}

	err := s.taskRepo.CreateTaskTX(task, taskRun)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Scheduler) assignTask(taskKey string) error {
	workerID, err := s.selectWorkerID()
	if err != nil {
		return err
	}

	nextStatus := model.TaskStatusRunning
	now := time.Now()
	err = s.taskRepo.UpdateTaskTX(
		&model.Task{
			TaskKey: taskKey,
			Status:  nextStatus.PreWaitStatus(),
		},
		&model.TaskRun{
			TaskKey:       taskKey,
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

		newAvailableWorkers := s.getAvailableWorkers()
		taskRuns, err := s.taskRepo.ListTaskRuns()
		if err != nil {
			log.Error("获取任务列表失败: %v", err)
			continue
		}
		for _, run := range taskRuns {
			found := slices.ContainsFunc(newAvailableWorkers, func(newWorker discover.Instance) bool {
				return newWorker.InstanceId == run.WorkerID
			})
			if !found {
				if run.WorkerID == "" {
					log.Error("任务[%d]首次分配工作者", run.ID)
				} else {
					log.Error("任务[%d]需要重新分配, 工作者[%s]已下线", run.ID, run.WorkerID)
				}
				if err := s.assignTask(run.TaskKey); err != nil {
					log.Error("任务[%d]分配失败, err: %v", run.ID, err)
				}
			}
		}
	}
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
	InstanceId string
	Score      float64
}

func (s *Scheduler) selectWorkerID() (string, error) {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()

	availableWorkers := s.getAvailableWorkers()
	if len(availableWorkers) == 0 {
		return "", errors.New("没有可用的 worker 服务")
	}

	selectedWorker, err := selectWorkerByResources(availableWorkers)
	if err != nil {
		return "", err
	}

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
			if newWorker.InstanceId == oldWorker.InstanceId {
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

func (s *Scheduler) updateLocalResourceEstimate(worker discover.Instance) {
	resourceUsage := ParseResourceUsage(worker.Metadata)
	cpuUsage := resourceUsage[cpuUsageKey]
	memUsage := resourceUsage[memUsageKey]
	goroutineNum := resourceUsage[goGoroutineKey]

	worker.Metadata[cpuUsageKey] = strconv.FormatFloat(cpuUsage+0.05, 'f', 2, 64)
	worker.Metadata[memUsageKey] = strconv.FormatFloat(memUsage+0.05, 'f', 2, 64)
	worker.Metadata[goGoroutineKey] = strconv.FormatFloat(goroutineNum+1, 'f', 2, 64)

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

func (s *Scheduler) setAvailableWorkers(instances []discover.Instance) {
	s.availableWorkers.Store(instances)
}

func (s *Scheduler) getAvailableWorkers() []discover.Instance {
	workers := s.availableWorkers.Load().([]discover.Instance)
	newWorkers := make([]discover.Instance, len(workers))
	copy(newWorkers, workers)
	return newWorkers
}

func selectWorkerByResources(workers []discover.Instance) (discover.Instance, error) {
	if len(workers) == 0 {
		return discover.Instance{}, errors.New("没有可用的 worker")
	}
	if len(workers) == 1 {
		return workers[0], nil
	}

	machineScores := make([]workerScore, 0, len(workers))
	for _, worker := range workers {
		resourceUsage := ParseResourceUsage(worker.Metadata)
		cpuScore := resourceUsage[cpuUsageKey]
		memoryScore := resourceUsage[memUsageKey]

		machineScores = append(machineScores, workerScore{
			InstanceId: worker.InstanceId,
			Score:      cpuScore*0.5 + memoryScore*0.5,
		})
	}
	sort.SliceStable(machineScores, func(i, j int) bool {
		return machineScores[i].Score < machineScores[j].Score
	})

	// 对于机器资源分数接近的节点，通过进程资源分数进一步筛选
	candidateWorkers := make([]discover.Instance, 0, len(machineScores))
	for i, score := range machineScores {
		if i != 0 && score.Score-machineScores[i-1].Score > 3 {
			break
		}
		candidateWorkers = append(candidateWorkers, discover.Instance{InstanceId: score.InstanceId})
	}
	if len(candidateWorkers) == 1 {
		return candidateWorkers[0], nil
	}

	var (
		bestWorker discover.Instance
		bestScore  float64 = -1
	)
	for _, candidate := range candidateWorkers {
		for _, worker := range workers {
			if worker.InstanceId == candidate.InstanceId {
				resourceUsage := ParseResourceUsage(worker.Metadata)
				goroutineNum := resourceUsage[goGoroutineKey]
				gcPause := resourceUsage[goGcPauseKey]
				gcCount := resourceUsage[goGcCountKey]

				var programScore float64
				if gcCount == 0 {
					programScore = float64(goroutineNum)
				} else {
					programScore = float64(goroutineNum)*0.5 + float64(gcPause)/float64(gcCount)*0.5
				}

				if bestScore == -1 || programScore < bestScore {
					bestScore = programScore
					bestWorker = worker
				}
				break
			}
		}
	}

	return bestWorker, nil
}
