package mysql

import (
	"context"
	"errors"
	"time"

	"github.com/samber/lo"
	"github.com/xyzbit/minitaskx/core/model"
	"github.com/xyzbit/minitaskx/core/taskrepo"
	"gorm.io/gorm"
)

type taskRepoImpl struct {
	db *gorm.DB
}

func NewTaskRepo(db *gorm.DB) taskrepo.Interface {
	return &taskRepoImpl{
		db: db,
	}
}

func (t *taskRepoImpl) CreateTaskTX(ctx context.Context, task *model.Task, taskRun *model.TaskRun) error {
	taskPo := FromTaskModel(task)
	taskRunPo := FromTaskRunModel(taskRun)

	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(taskPo).Error; err != nil {
			return err
		}
		return tx.Create(taskRunPo).Error
	})
}

func (t *taskRepoImpl) UpdateTaskTX(ctx context.Context, task *model.Task, taskRun *model.TaskRun) error {
	if task.TaskKey == "" {
		return errors.New("task key is empty")
	}

	taskPo := FromTaskModel(task)
	taskRunPo := FromTaskRunModel(taskRun)
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Task{}).
			Where("task_key = ?", task.TaskKey).
			Updates(&taskPo).Error; err != nil {
			return err
		}
		return tx.Model(&TaskRun{}).
			Where("task_key = ?", task.TaskKey).
			Updates(&taskRunPo).Error
	})
}

func (t *taskRepoImpl) UpdateTaskStatus(ctx context.Context, taskKey string, status model.TaskStatus, result string) error {
	return t.db.Model(&Task{}).
		Where("task_key = ?", taskKey).
		Updates(map[string]interface{}{
			"status": status.String(),
			"result": result,
		}).Error
}

func (t *taskRepoImpl) GetTask(ctx context.Context, taskKey string) (*model.Task, error) {
	var task Task
	if err := t.db.WithContext(ctx).
		Model(&Task{}).
		Where("task_key = ?", taskKey).
		First(&task).Error; err != nil {
		return nil, err
	}
	return ToTaskModel(&task), nil
}

func (t *taskRepoImpl) BatchGetTask(ctx context.Context, taskKeys []string) ([]*model.Task, error) {
	if len(taskKeys) == 0 {
		return nil, nil
	}
	var tasks []*Task
	if err := t.db.WithContext(ctx).
		Model(&Task{}).
		Where("task_key in (?)", taskKeys).
		Find(&tasks).Error; err != nil {
		return nil, err
	}

	return lo.Map(tasks, func(item *Task, index int) *model.Task {
		return ToTaskModel(item)
	}), nil
}

func (t *taskRepoImpl) ListTaskRuns(ctx context.Context) ([]*model.TaskRun, error) {
	var taskRuns []*TaskRun
	if err := t.db.WithContext(ctx).
		Model(&TaskRun{}).
		Find(&taskRuns).Error; err != nil {
		return nil, err
	}

	return lo.Map(taskRuns, func(item *TaskRun, index int) *model.TaskRun {
		return ToTaskRunModel(item)
	}), nil
}

func (t *taskRepoImpl) FinshTaskTX(ctx context.Context, taskKey string, status model.TaskStatus, failedReason string) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Task{}).
			Where("task_key = ?", taskKey).
			Update("status", status.String()).Error; err != nil {
			return err
		}
		return tx.Model(&TaskRun{}).
			Where("task_key = ?", taskKey).
			Delete(&TaskRun{}).Error
	})
}

func (t *taskRepoImpl) ListRunnableTasks(ctx context.Context, workerID string) ([]*model.TaskRun, error) {
	var taskRuns []*TaskRun
	err := t.db.WithContext(ctx).Model(&TaskRun{}).
		Where("worker_id = ?", workerID).
		Where("next_run_at <= ?", time.Now()).
		Find(&taskRuns).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(taskRuns, func(item *TaskRun, index int) *model.TaskRun {
		return ToTaskRunModel(item)
	}), nil
}
