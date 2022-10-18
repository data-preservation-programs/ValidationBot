package observer

import (
	"context"
	"encoding/json"

	"validation-bot/module"

	"validation-bot/store"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Observer struct {
	db               *gorm.DB
	trustedPeers     []peer.ID
	lastCids         []*cid.Cid
	resultSubscriber store.ResultSubscriber
	log              zerolog.Logger
}

func NewObserver(db *gorm.DB,
	resultSubscriber store.ResultSubscriber,
	peers []peer.ID,
) (*Observer, error) {
	cids := make([]*cid.Cid, len(peers))

	err := db.AutoMigrate(&module.ValidationResultModel{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to migrate types")
	}

	return &Observer{
		db:               db,
		trustedPeers:     peers,
		lastCids:         cids,
		resultSubscriber: resultSubscriber,
		log:              log.With().Str("role", "observer").Logger(),
	}, nil
}

func (o Observer) Start(ctx context.Context) <-chan error {
	log := o.log
	errChannel := make(chan error)
	for i, peerID := range o.trustedPeers {
		i, peerID := i, peerID
		go func() {
			last := o.lastCids[i]
			log.Info().Str("peer", peerID.String()).Msg("start listening to subscription")
			entries, err := o.resultSubscriber.Subscribe(ctx, peerID, last)
			if err != nil {
				log.Error().Err(err).Msg("failed to receive next message")
				return
			}
			for {
				select {
				case <-ctx.Done():
					return
				case entry := <-entries:
					log.Info().Str("from", peerID.String()).Bytes("message", entry.Message).
						Interface("cid", entry.Previous).Msg("storing received message")
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
	result := &module.ValidationResult{}
	err := json.Unmarshal(data, result)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal to concrete type")
	}

	toStore := module.ValidationResultModel{
		Task:   result.Task,
		Result: result.Result,
	}
	err = o.db.WithContext(ctx).Create(&toStore).Error
	if err != nil {
		return errors.Wrap(err, "failed to store concrete result")
	}

	return nil
}
