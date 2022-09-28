package dispatcher

import (
	"context"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Dispatcher interface {
	Start(ctx context.Context) error
	Create(ctx context.Context, taskDef *task.Definition) error
	Remove(ctx context.Context, id uuid.UUID) error
}

type Group struct {
	db            *gorm.DB
	taskPublisher task.Publisher
	dispatcherMap map[task.Type]Dispatcher
}

type Config struct {
	Db            *gorm.DB
	TaskPublisher task.Publisher
}

func NewDispatcherGroup(ctx context.Context, config Config) (*Group, error) {
	db := config.Db.WithContext(ctx)
	err := db.AutoMigrate(&task.Definition{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot migrate task definitions")
	}

	dispatcherMap := map[task.Type]Dispatcher{}
	dispatcherMap[task.EchoType] = EchoDispatcher{Db: db, Publisher: config.TaskPublisher}

	return &Group{
		db:            db,
		taskPublisher: config.TaskPublisher,
		dispatcherMap: dispatcherMap,
	}, nil
}

func (g Group) Start(ctx context.Context) error {
	for _, dispatcher := range g.dispatcherMap {
		err := dispatcher.Start(ctx)
		if err != nil {
			return errors.Wrap(err, "cannot start dispatcher")
		}
	}

	return nil
}

func (g Group) List(ctx context.Context) ([]task.Definition, error) {
	var taskDefs []task.Definition
	err := g.db.Find(&taskDefs).Error
	if err != nil {
		return nil, errors.Wrap(err, "cannot fetch task definitions")
	}

	return taskDefs, nil
}

func (g Group) Create(ctx context.Context, taskDef *task.Definition) error {
	dispatcher, ok := g.dispatcherMap[taskDef.Type]
	if !ok {
		return errors.Errorf("unknown task type %s", taskDef.Type)
	}

	err := dispatcher.Create(ctx, taskDef)
	if err != nil {
		return errors.Wrap(err, "cannot create task definition")
	}

	return nil
}

func (g Group) Remove(ctx context.Context, id uuid.UUID) error {
	taskDef, err := FindTask(g.db, id)
	if err != nil {
		return errors.Wrap(err, "cannot find task definition")
	}

	if taskDef == nil {
		return nil
	}

	dispatcher, ok := g.dispatcherMap[taskDef.Type]
	if !ok {
		return errors.Errorf("unknown task type %s", taskDef.Type)
	}

	err = dispatcher.Remove(ctx, id)
	if err != nil {
		return errors.Wrap(err, "cannot remove task definition")
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
