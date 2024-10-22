package mysql

import (
	"errors"

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

func (t *taskRepoImpl) CreateTaskTX(task *model.Task, taskRun *model.TaskRun) error {
	return t.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		return tx.Create(taskRun).Error
	})
}

func (t *taskRepoImpl) UpdateTaskTX(task *model.Task, taskRun *model.TaskRun) error {
	if task.TaskKey == "" {
		return errors.New("task key is empty")
	}
	return t.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Task{}).
			Where("task_key = ?", task.TaskKey).
			Updates(&task).Error; err != nil {
			return err
		}
		return tx.Model(&model.TaskRun{}).
			Where("task_key = ?", task.TaskKey).
			Updates(&taskRun).Error
	})
}

func (t *taskRepoImpl) GetTask(taskKey string) (*model.Task, error) {
	var task model.Task
	if err := t.db.Model(&Task{}).Where("task_key = ?", taskKey).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (t *taskRepoImpl) ListTaskRuns() ([]*model.TaskRun, error) {
	var taskRuns []*model.TaskRun
	if err := t.db.Model(&TaskRun{}).Find(&taskRuns).Error; err != nil {
		return nil, err
	}
	return taskRuns, nil
}
