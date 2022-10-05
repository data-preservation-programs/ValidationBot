package dispatcher

import (
	"context"
	"fmt"
	"time"

	"validation-bot/module"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Dispatcher struct {
	db            *gorm.DB
	taskPublisher task.Publisher
	modules       map[task.Type]module.Module
	checkInterval time.Duration
}

type Config struct {
	Db            *gorm.DB
	TaskPublisher task.Publisher
	Modules       []module.Module
	CheckInterval time.Duration
}

func NewDispatcher(config Config) (*Dispatcher, error) {
	db := config.Db

	modules := make(map[task.Type]module.Module)
	for _, mod := range config.Modules {
		modules[mod.TaskType()] = mod
	}

	return &Dispatcher{
		db:            db,
		taskPublisher: config.TaskPublisher,
		modules:       modules,
		checkInterval: config.CheckInterval,
	}, nil
}

func (g Dispatcher) Start(ctx context.Context) <-chan error {
	errChannel := make(chan error)
	for _, mod := range g.modules {
		mod := mod
		go func() {
			for {
				var defs []task.Definition
				err := g.db.WithContext(ctx).Model(&task.Definition{}).
					Where("type = ? AND interval_seconds > 0 AND updated_at + interval_seconds * interval '1 second' < now()",
						mod.TaskType()).
					Find(&defs).Error
				if err != nil {
					errChannel <- errors.Wrap(err, "cannot fetch task definitions")
					return
				}

				tasks, err := mod.GetTasks(defs)
				if err != nil {
					errChannel <- errors.Wrap(err, "cannot get tasks to dispatch")
					return
				}

				for def, input := range tasks {
					err = g.dispatchOnce(ctx, &def, input)
					if err != nil {
						errChannel <- errors.Wrap(err, "cannot dispatch task")
						return
					}
				}

				time.Sleep(g.checkInterval)
			}
		}()
	}

	return nil
}

func (g Dispatcher) dispatchOnce(ctx context.Context, def *task.Definition, input module.Marshallable) error {
	bytes, err := input.Marshal()
	if err != nil {
		return errors.Wrap(err, "cannot marshal task")
	}

	err = g.taskPublisher.Publish(ctx, bytes)
	fmt.Println((string)(bytes))
	if err != nil {
		return errors.Wrap(err, "cannot publish task")
	}

	err = g.db.WithContext(ctx).Model(def).Where("id = ?", def.ID).
		Update("dispatched_times", def.DispatchedTimes+1).Error
	if err != nil {
		return errors.Wrap(err, "cannot increment dispatched_times")
	}

	return nil
}

func (g Dispatcher) List(ctx context.Context) ([]task.Definition, error) {
	var taskDefs []task.Definition
	err := g.db.WithContext(ctx).Find(&taskDefs).Error
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch task definitions")
	}

	return taskDefs, nil
}

func (g Dispatcher) Create(ctx context.Context, taskDef *task.Definition) error {
	mod, ok := g.modules[taskDef.Type]
	if !ok {
		return errors.Errorf("unknown task type %s", taskDef.Type)
	}

	err := g.db.WithContext(ctx).Create(&taskDef).Error
	if err != nil {
		return errors.Wrap(err, "cannot create task definition")
	}

	if taskDef.IntervalSeconds == 0 {
		input, err := mod.GetTask(*taskDef)
		if err != nil {
			return errors.Wrap(err, "cannot get task to dispatch")
		}

		err = g.dispatchOnce(ctx, taskDef, input)
		if err != nil {
			return errors.Wrap(err, "cannot dispatch task")
		}
	}

	return nil
}

func (g Dispatcher) Remove(ctx context.Context, id uuid.UUID) error {
	err := g.db.WithContext(ctx).Delete(&task.Definition{}, id).Error
	if err != nil {
		return errors.Wrap(err, "cannot delete task definition from the database")
	}

	return nil
}

func FindTask(db *gorm.DB, id uuid.UUID) (*task.Definition, error) {
	var taskDef task.Definition
	result := db.First(&taskDef, id)

	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Info().Str("id", id.String()).Msg("task definition removed")
		return nil, nil
	} else if result.Error != nil {
		log.Error().Err(result.Error).Str("id", id.String()).Msg("cannot fetch task definition")
		return nil, result.Error
	}

	return &taskDef, nil
}
