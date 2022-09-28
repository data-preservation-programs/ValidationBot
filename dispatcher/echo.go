package dispatcher

import (
	"context"
	"encoding/json"
	"time"

	"validation-bot/task"

	"validation-bot/auditor/echo"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type EchoDispatcher struct {
	Db        *gorm.DB
	Publisher task.Publisher
}

func (d EchoDispatcher) Start(ctx context.Context) error {
	var defs []task.Definition

	err := d.Db.Model(&task.Definition{}).Where("type = ? AND interval_seconds > 0", task.EchoType).Find(&defs).Error
	if err != nil {
		return errors.Wrap(err, "cannot fetch echo task definitions")
	}

	for _, def := range defs {
		def := def
		d.dispatchContinuously(ctx, &def)
	}

	return nil
}

func (d EchoDispatcher) Create(ctx context.Context, taskDef *task.Definition) error {
	// For echo task, there is no restriction on how many tasks can be created for a single provider
	err := d.Db.Create(taskDef).Error
	if err != nil {
		return errors.Wrap(err, "cannot create task definition in the database")
	}

	if taskDef.IntervalSeconds == 0 {
		err = d.dispatchOnce(ctx, taskDef)
		if err != nil {
			return errors.Wrap(err, "cannot dispatch one-off task")
		}
	} else {
		d.dispatchContinuously(ctx, taskDef)
	}

	return nil
}

func (d EchoDispatcher) Remove(ctx context.Context, id uuid.UUID) error {
	err := d.Db.Delete(&task.Definition{}, id).Error
	if err != nil {
		return errors.Wrap(err, "cannot delete task definition from the database")
	}

	return nil
}

func (d EchoDispatcher) dispatchContinuously(ctx context.Context, taskDef *task.Definition) {
	log.Info().Str("id", taskDef.ID.String()).Msg("dispatching echo task")

	go func() {
		future := time.Until(taskDef.UpdatedAt.Add(time.Duration(taskDef.IntervalSeconds) * time.Second))

		if future > 0 {
			time.Sleep(future)
		}

		for {
			def, err := FindTask(d.Db, taskDef.ID)
			if def != nil && err != nil {
				err = d.dispatchOnce(ctx, def)
				if err != nil {
					log.Error().Err(err).Str("id", def.ID.String()).Msg("cannot dispatch echo task")
				}

				time.Sleep(time.Duration(def.IntervalSeconds) * time.Second)
			}
		}
	}()
}

func (d EchoDispatcher) dispatchOnce(ctx context.Context, taskDef *task.Definition) error {
	log.Info().Str("id", taskDef.ID.String()).Msg("dispatching echo task")

	commonTask := task.Task{
		Type:   taskDef.Type,
		Target: taskDef.Target,
	}
	echoTask := echo.Task{
		Task:  commonTask,
		Input: taskDef.Definition,
	}

	taskStr, err := json.Marshal(&echoTask)
	if err != nil {
		return errors.Wrap(err, "cannot marshal task")
	}

	err = d.Publisher.Publish(ctx, taskStr)
	if err != nil {
		return errors.Wrap(err, "cannot publish echo task")
	}

	err = d.Db.Model(&task.Definition{}).Where("id = ?", taskDef.ID).
		Update("dispatched_times", taskDef.DispatchedTimes+1).Error
	if err != nil {
		log.Error().Err(err).Str("id", taskDef.ID.String()).Msg("cannot update dispatched times")
	}

	return nil
}
