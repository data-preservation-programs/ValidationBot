package dispatcher

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"time"

	"validation-bot/module"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Dispatcher struct {
	db            *gorm.DB
	taskPublisher task.Publisher
	modules       map[task.Type]module.DispatcherModule
	checkInterval time.Duration
	log           zerolog.Logger
	jitter        time.Duration
}

type Config struct {
	DB            *gorm.DB
	TaskPublisher task.Publisher
	Modules       map[task.Type]module.DispatcherModule
	CheckInterval time.Duration
	Jitter        time.Duration
}

func NewDispatcher(config Config) (*Dispatcher, error) {
	db := config.DB

	return &Dispatcher{
		db:            db,
		taskPublisher: config.TaskPublisher,
		modules:       config.Modules,
		checkInterval: config.CheckInterval,
		log:           log2.With().Str("role", "dispatcher").Caller().Logger(),
		jitter:        config.Jitter,
	}, nil
}

//nolint:lll
func (g Dispatcher) Start(ctx context.Context) <-chan error {
	errChannel := make(chan error)

	for modName, mod := range g.modules {
		modName, mod := modName, mod
		log := g.log.With().Str("moduleName", modName).Logger()

		go func() {
			for {
				log.Debug().Msg("polling task definitions")

				defs := make([]task.Definition, 0)

				err := g.db.WithContext(ctx).Model(&task.Definition{}).
					Where(
						"type = ? AND interval_seconds > 0 AND (dispatched_times = 0 OR updated_at + interval_seconds * interval '1 second' < ?)",
						modName, time.Now(),
					).
					Find(&defs).Error
				if err != nil {
					errChannel <- errors.Wrap(err, "cannot fetch task definitions")
					return
				}

				log.Info().Int("size", len(defs)).Msg("polled task definitions in ready state")

				tasks, err := mod.GetTasks(defs)
				if err != nil {
					errChannel <- errors.Wrap(err, "cannot get tasks to dispatch")
					return
				}

				log.Info().Int("size", len(tasks)).Msg("generated tasks to be published")

				for id, input := range tasks {
					err = g.dispatchOnce(ctx, id, input)
					if err != nil {
						errChannel <- errors.Wrap(err, "cannot dispatch task")
						return
					}
				}

				log.Debug().Dur("sleep", g.checkInterval).Msg("sleeping")
				time.Sleep(g.checkInterval)
			}
		}()
	}

	return errChannel
}

func (g Dispatcher) additionalJitter() time.Duration {
	rnd, err := rand.Int(rand.Reader, big.NewInt(g.jitter.Nanoseconds()))
	if err != nil {
		panic(err)
	}

	return time.Duration(rnd.Int64()) * time.Nanosecond
}

func (g Dispatcher) dispatchOnce(ctx context.Context, definitionID uuid.UUID, input module.ValidationInput) error {
	g.log.Info().Str("moduleName", input.Task.Type).
		Str("id", definitionID.String()).Interface("task", input).Msg("dispatching task")

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
		time.Now().Add(g.additionalJitter()),
		definitionID,
	).Error
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
