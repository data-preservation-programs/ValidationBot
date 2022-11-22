package observer

import (
	"context"
	"encoding/json"
	"time"

	"validation-bot/module"
	"validation-bot/role/trust"

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
	trustManager     *trust.Manager
	resultSubscriber store.Subscriber
	log              zerolog.Logger
}

func NewObserver(
	db *gorm.DB,
	trustManager *trust.Manager,
	resultSubscriber store.Subscriber,
) (*Observer, error) {
	err := db.AutoMigrate(&module.ValidationResultModel{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to migrate validation result model")
	}

	return &Observer{
		db:               db,
		trustManager:     trustManager,
		resultSubscriber: resultSubscriber,
		log:              log2.With().Str("role", "observer").Caller().Logger(),
	}, nil
}

func (o *Observer) lastCidFromDB(peer peer.ID) (*cid.Cid, error) {
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

func (o *Observer) Start(ctx context.Context) {
	o.trustManager.Start(ctx)

	currentTrustees := map[peer.ID]struct{}{}

	go func() {
		for {
			newTrustees := o.trustManager.Trustees()
			for peerID := range newTrustees {
				if _, ok := currentTrustees[peerID]; !ok {
					go o.downloadEntriesForAuditorPeer(ctx, peerID)
				}
			}

			currentTrustees = newTrustees

			time.Sleep(time.Second)
		}
	}()
}

func (o *Observer) downloadEntriesForAuditorPeer(ctx context.Context, peerID peer.ID) {
	log := o.log.With().Str("peer", peerID.String()).Logger()

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
		if !o.trustManager.IsTrusted(peerID) {
			log.Warn().Msg("peer has been revoked")
			return
		}

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
}

func (o *Observer) storeResult(ctx context.Context, data []byte, cid cid.Cid, peerID peer.ID, previous *cid.Cid) error {
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
