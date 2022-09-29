package observer

import (
	"context"
	"encoding/json"

	"validation-bot/module"

	"validation-bot/task"

	"validation-bot/store"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Observer struct {
	Db               *gorm.DB
	TrustedPeers     []peer.ID
	LastCids         []*cid.Cid
	ResultSubscriber store.ResultSubscriber
	Modules          map[string]module.Module
}

func NewObserver(db *gorm.DB,
	resultSubscriber store.ResultSubscriber,
	peers []peer.ID,
	modules []module.Module,
) (*Observer, error) {
	cids := make([]*cid.Cid, len(peers))

	mods := make(map[string]module.Module)
	for _, mod := range modules {
		mods[mod.TaskType()] = mod
		err := db.AutoMigrate(mod.ResultType())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to migrate type from %s", mod.TaskType())
		}
	}

	return &Observer{
		Db:               db,
		TrustedPeers:     peers,
		LastCids:         cids,
		ResultSubscriber: resultSubscriber,
		Modules:          mods,
	}, nil
}

func (o Observer) Start(ctx context.Context) <-chan error {
	errChannel := make(chan error)
	for i, peerID := range o.TrustedPeers {
		i, peerID := i, peerID
		go func() {
			last := o.LastCids[i]
			entries, err := o.ResultSubscriber.Subscribe(ctx, peerID, last)
			if err != nil {
				log.Error().Err(err).Msg("failed to receive next message")
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case entry := <-entries:
					log.Info().Str("from", peerID.String()).
						Interface("cid", entry.PreviousCid.String()).Msg("received message")
					err = o.storeResult(ctx, entry.Message)
					if err != nil {
						errChannel <- errors.Wrap(err, "failed to store result")
					}
				}
			}
		}()
	}

	return errChannel
}

func (o Observer) storeResult(ctx context.Context, data []byte) error {
	entryTask := new(task.Task)
	err := json.Unmarshal(data, entryTask)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal data to get the type")
	}

	mod, ok := o.Modules[entryTask.Type]
	if !ok {
		return errors.Errorf("unknown task type %s", entryTask.Type)
	}

	result := mod.ResultType()
	err = json.Unmarshal(data, result)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal to concrete type")
	}

	err = o.Db.WithContext(ctx).Create(result).Error
	if err != nil {
		return errors.Wrap(err, "failed to store concrete result")
	}

	return nil
}
