package trust

import (
	"context"
	"encoding/json"

	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

type (
	Revoked       = bool
	PeerIDStr     = string
	OperationType = string
)

const (
	Create OperationType = "create"
	Revoke OperationType = "revoke"
)

type Operation struct {
	Type OperationType `json:"type"`
	Peer PeerIDStr     `json:"peer"`
}

func AddNewPeer(ctx context.Context, publisher store.ResultPublisher, peerID PeerIDStr) error {
	operation := Operation{
		Type: Create,
		Peer: peerID,
	}

	marshal, err := json.Marshal(operation)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal operation")
	}

	err = publisher.Publish(ctx, marshal)
	if err != nil {
		return errors.Wrapf(err, "failed to publish operation")
	}

	return nil
}

func RevokePeer(ctx context.Context, publisher store.ResultPublisher, peerID PeerIDStr) error {
	operation := Operation{
		Type: Revoke,
		Peer: peerID,
	}

	marshal, err := json.Marshal(operation)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal operation")
	}

	err = publisher.Publish(ctx, marshal)
	if err != nil {
		return errors.Wrapf(err, "failed to publish operation")
	}

	return nil
}

func ListPeers(ctx context.Context, subscriber store.ResultSubscriber, peerID PeerIDStr) (
	map[PeerIDStr]Revoked,
	error,
) {
	decode, err := peer.Decode(peerID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode peer id %s", peerID)
	}

	entries, err := subscriber.Subscribe(ctx, decode, nil, true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe to peer %s", peerID)
	}

	result := make(map[PeerIDStr]Revoked)

	for entry := range entries {
		var operation Operation

		err = json.Unmarshal(entry.Message, &operation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal operation")
		}

		switch operation.Type {
		case Create:
			result[operation.Peer] = true
		case Revoke:
			result[operation.Peer] = false
		default:
			return nil, errors.Errorf("unknown operation type %s", operation.Type)
		}
	}

	return result, nil
}
