package main

import (
	"context"
	"fmt"
	"github.com/application-research/filclient/rep"
	"github.com/application-research/filclient/retrievehelper"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/dustin/go-humanize"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/ipfs/go-cid"
	"github.com/mitchellh/go-homedir"
	"os"
	"time"
)

type ValidationEvent struct {
	Cid      string `json:"cid"`
	Provider string `json:"provider"`
}

type RetrievalSubscriber struct {
	rep.RetrievalSubscriber
}

func (r RetrievalSubscriber) OnRetrievalEvent(event rep.RetrievalEvent) {
	fmt.Printf("[%s]RetrievalEvent - code: %s, phase: %s\n", time.Now(), event.Code(), event.Phase())
}

func HandleRequest(ctx context.Context, event ValidationEvent) (string, error) {
	c, err := cid.Decode(event.Cid)
	if err != nil {
		return "", fmt.Errorf("failed to decode cid %s: %w", event.Cid, err)
	}

	provider, err := address.NewFromString(event.Provider)
	if err != nil {
		return "", fmt.Errorf("failed to parse provider address %s: %w", event.Provider, err)
	}

	ddir, err := homedir.Expand("~/.filc")
	if err != nil {
		return "", fmt.Errorf("failed to expand home dir: %w", err)
	}

	if err := os.MkdirAll(ddir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data dir %s: %w", ddir, err)
	}

	node, err := setup(ctx, ddir)
	if err != nil {
		return "", fmt.Errorf("failed to setup node: %w", err)
	}

	fc, closer, err := clientFromNode(ctx, node, ddir)
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	defer closer()

	query, err := fc.RetrievalQuery(ctx, provider, c)
	fc.SubscribeToDataTransferEvents(func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		fmt.Printf("[%s]DataTransfer - code: %s,  channelstatus: %s, received: %d - %s%s\n", event.Timestamp,
			datatransfer.Events[event.Code], datatransfer.Statuses[channelState.Status()], channelState.Received(), event.Message, channelState.Message())
	})
	fc.SubscribeToRetrievalEvents(RetrievalSubscriber{})
	if err != nil {
		return "", fmt.Errorf("failed to query retrieval with provider %s and cid %s: %w", provider, c, err)
	}

	proposal, err := retrievehelper.RetrievalProposalForAsk(query, c, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create proposal for ask: %w", err)
	}

	stats, err := fc.RetrieveContentWithProgressCallback(
		ctx,
		provider,
		proposal,
		func(bytesReceived uint64) {
			fmt.Printf("[%s]Received %s\n", time.Now(), humanize.Bytes(bytesReceived))
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to retrieve content: %w", err)
	}

	fmt.Printf("%+v\n", stats)

	return "", nil
}

func main() {
	lambda.Start(HandleRequest)
}
