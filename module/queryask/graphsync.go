package queryask

import (
	"context"

	"github.com/application-research/filclient"
	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"
)

type GraphSyncRetriever struct {
	log       zerolog.Logger
	filclient *filclient.FilClient
}

func (g GraphSyncRetriever) Retrieve(ctx context.Context, provider address.Address, dataCid cid.Cid) (*ResultContent, error) {
	panic("not implemented")
}
