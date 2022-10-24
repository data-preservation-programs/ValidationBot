package dispatcher

import (
	"context"
	"encoding/json"
	"time"

	"validation-bot/module"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Dispatcher struct {
	db            *gorm.DB
	taskPublisher task.Publisher
	modules       map[task.Type]module.DispatcherModule
	checkInterval time.Duration
	log           zerolog.Logger
}

type Config struct {
	DB            *gorm.DB
	TaskPublisher task.Publisher
	Modules       map[task.Type]module.DispatcherModule
	CheckInterval time.Duration
}

func NewDispatcher(config Config) (*Dispatcher, error) {
	db := config.DB

	return &Dispatcher{
		db:            db,
		taskPublisher: config.TaskPublisher,
		modules:       config.Modules,
		checkInterval: config.CheckInterval,
		log:           log.With().Str("role", "dispatcher").Logger(),
	}, nil
}

func (g Dispatcher) Start(ctx context.Context) <-chan error {
	errChannel := make(chan error)
	for modName, mod := range g.modules {
		mod := mod
		go func() {
			log := g.log.With().Str("module", modName).Logger()
			for {
				var defs []task.Definition
				log.Info().Msg("polling task definitions")
				err := g.db.WithContext(ctx).Model(&task.Definition{}).
					Where("type = ? AND interval_seconds > 0 AND updated_at + interval_seconds * interval '1 second' < ?",
						modName, time.Now()).
					Find(&defs).Error
				if err != nil {
					errChannel <- errors.Wrap(err, "cannot fetch task definitions")
					return
				}

				log.Info().Int("size", len(defs)).Msg("polled task definitions in ready state")
				tasks, err := mod.GetTasks(defs)
				log.Info().Int("size", len(tasks)).Msg("generated tasks to be published")
				if err != nil {
					errChannel <- errors.Wrap(err, "cannot get tasks to dispatch")
					return
				}

				for id, input := range tasks {
					log.Info().Str("id", id.String()).Interface("task", input).Msg("dispatching task")
					err = g.dispatchOnce(ctx, id, input)
					if err != nil {
						errChannel <- errors.Wrap(err, "cannot dispatch task")
						return
					}
				}

				log.Info().Dur("sleep", g.checkInterval).Msg("sleeping")
				time.Sleep(g.checkInterval)
			}
		}()
	}

	return nil
}

func (g Dispatcher) dispatchOnce(ctx context.Context, definitionID uuid.UUID, input module.ValidationInput) error {
	bytes, err := json.Marshal(input)
	if err != nil {
		return errors.Wrap(err, "cannot marshal task input")
	}

	err = g.taskPublisher.Publish(ctx, bytes)
	if err != nil {
		return errors.Wrap(err, "cannot publish task")
	}

	err = g.db.WithContext(ctx).Exec(
		"UPDATE definitions SET dispatched_times = dispatched_times + 1, updated_at = ? WHERE id = ?",
		time.Now(),
		definitionID).Error
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

	err := mod.Validate(*taskDef)
	if err != nil {
		return errors.Wrap(err, "the task definition is invalid")
	}

	err = g.db.WithContext(ctx).Create(&taskDef).Error
	if err != nil {
		return errors.Wrap(err, "cannot create task definition")
	}

	if taskDef.IntervalSeconds == 0 {
		input, err := mod.GetTask(*taskDef)
		if err != nil {
			return errors.Wrap(err, "cannot get task to dispatch")
		}

		err = g.dispatchOnce(ctx, taskDef.ID, input)
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
