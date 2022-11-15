package trust

import (
	"context"
	"encoding/json"

	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

type (
	Valid         = bool
	OperationType = string
)

const (
	Create OperationType = "create"
	Revoke OperationType = "revoke"
)

type Operation struct {
	Type OperationType `json:"type"`
	Peer string        `json:"peer"`
}

func AddNewPeer(ctx context.Context, publisher store.ResultPublisher, peerID peer.ID) error {
	operation := Operation{
		Type: Create,
		Peer: peerID.String(),
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

func RevokePeer(ctx context.Context, publisher store.ResultPublisher, peerID peer.ID) error {
	operation := Operation{
		Type: Revoke,
		Peer: peerID.String(),
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

func ListPeers(ctx context.Context, subscriber store.ResultSubscriber, peerID peer.ID) (
	map[peer.ID]Valid,
	error,
) {
	entries, err := subscriber.Subscribe(ctx, peerID, nil, true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe to peer %s", peerID)
	}

	result := make(map[peer.ID]Valid)

	for entry := range entries {
		var operation Operation

		err = json.Unmarshal(entry.Message, &operation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal operation")
		}

		operationPeerID, err := peer.Decode(operation.Peer)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode peer")
		}

		switch operation.Type {
		case Create:
			result[operationPeerID] = true
		case Revoke:
			result[operationPeerID] = false
		default:
			return nil, errors.Errorf("unknown operation type %s", operation.Type)
		}
	}

	return result, nil
}
