package observer

import (
	"context"
	"encoding/json"

	"validation-bot/auditor/echo"

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
}

func NewObserver(db *gorm.DB, resultSubscriber store.ResultSubscriber, peerCidMap map[peer.ID]*cid.Cid) (*Observer, error) {
	peers := make([]peer.ID, 0, len(peerCidMap))
	cids := make([]*cid.Cid, 0, len(peerCidMap))
	for peerID, lastCid := range peerCidMap {
		peers = append(peers, peerID)
		cids = append(cids, lastCid)
	}

	err := db.AutoMigrate(&EchoResult{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to migrate echo result")
	}

	return &Observer{
		Db:               db,
		TrustedPeers:     peers,
		LastCids:         cids,
		ResultSubscriber: resultSubscriber,
	}, nil
}

func (o Observer) Start(ctx context.Context) {
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
					log.Info().Str("from", peerID.String()).Interface("cid", entry.Cid.String()).Msg("received message")
					err := o.storeResult(ctx, entry)
					if err != nil {
						log.Error().Err(err).Msg("failed to handle result")
						return
					}
				}
			}
		}()
	}
}

type EchoResult struct {
	gorm.Model
	echo.Result
}

func (o Observer) storeResult(ctx context.Context, entry store.Entry) error {
	entryTask := new(task.Task)
	err := json.Unmarshal(entry.Message, entryTask)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal task")
	}

	switch entryTask.Type {
	case task.EchoType:
		result := new(echo.Result)

		err = json.Unmarshal(entry.Message, result)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal echo result")
		}

		echoResult := EchoResult{
			Result: *result,
		}

		err = o.Db.Create(echoResult).Error
		if err != nil {
			return errors.Wrap(err, "failed to store echo result")
		}
	default:
		return errors.Errorf("unknown task type: %s", entryTask.Type)
	}

	return nil
}
