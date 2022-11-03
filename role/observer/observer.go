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
	log2 "github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Observer struct {
	db               *gorm.DB
	trustedPeers     []peer.ID
	resultSubscriber store.ResultSubscriber
	log              zerolog.Logger
}

func NewObserver(
	db *gorm.DB,
	resultSubscriber store.ResultSubscriber,
	peers []peer.ID,
) (*Observer, error) {
	err := db.AutoMigrate(&module.ValidationResultModel{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to migrate types")
	}
	return &Observer{
		db:               db,
		trustedPeers:     peers,
		resultSubscriber: resultSubscriber,
		log:              log2.With().Str("role", "observer").Caller().Logger(),
	}, nil
}

func (o Observer) lastCidFromDB(peer peer.ID) (*cid.Cid, error) {
	model := module.ValidationResultModel{}

	last := o.db.Last(&model, "peer_id = ?", peer.String())
	if last.Error != nil {
		if !errors.Is(last.Error, gorm.ErrRecordNotFound) {
			return nil, errors.Wrapf(last.Error, "failed to get last result for peer %s", peer.String())
		}

		return nil, nil
	}

	cid, err := cid.Decode(model.Cid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode cid %s", model.Cid)
	}
	return &cid, nil
}

func (o Observer) Start(ctx context.Context) {
	for _, peerID := range o.trustedPeers {
		peerID := peerID
		log := o.log.With().Str("peer", peerID.String()).Logger()

		go func() {
			last, err := o.lastCidFromDB(peerID)
			if err != nil {
				log.Error().Err(err).Msg("failed to get last cid from db")
				return
			}

			log.Info().Interface("lastCid", last).Msg("start listening to subscription")

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
					log.Info().Bytes("message", entry.Message).
						Interface("previous", entry.Previous).
						Str("cid", entry.CID.String()).Msg("storing received message")

					err = o.storeResult(ctx, entry.Message, entry.CID, peerID, entry.Previous)
					if err != nil {
						log.Error().Err(err).Msg("failed to store result")
					}
				}
			}
		}()
	}
}

func (o Observer) storeResult(ctx context.Context, data []byte, cid cid.Cid, peerID peer.ID, previous *cid.Cid) error {
	result := &module.ValidationResult{}

	err := json.Unmarshal(data, result)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal to concrete type")
	}

	var previousCid *string

	if previous != nil {
		p := previous.String()
		previousCid = &p
	}

	toStore := module.ValidationResultModel{
		ValidationInput: result.ValidationInput,
		Result:          result.Result,
		Cid:             cid.String(),
		PeerID:          peerID.String(),
		PreviousCid:     previousCid,
	}

	err = o.db.WithContext(ctx).Create(&toStore).Error
	if err != nil {
		return errors.Wrap(err, "failed to store concrete result")
	}

	return nil
}
