package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"time"
	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	bitswap "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-cid"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	routing2 "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
)

const (
	BITSWAP_PROTOCOL = "/fil/retrieval/1.0.0"
	CONTENT_ROUTING  = "dht"
)

type BitswapRetriever struct {
	log     zerolog.Logger
	libp2p  host.Host
	tmpDir  string
	bitswap *bitswap.Bitswap
	bs      blockstore.Blockstore
	routing routing2.ContentRouting
}

type BitswapRetrieverBuilder struct{}

func initDHT(ctx context.Context, libp2p host.Host) (routing2.ContentRouting, error) {
	dht, err := dht.New(ctx, libp2p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dht")
	}

	if err := dht.Bootstrap(ctx); err != nil {
		return nil, errors.Wrap(err, "cannot bootstrap dht")
	}

	return dht, nil
}

func (b *BitswapRetrieverBuilder) Build(ctx context.Context, dir string) (*BitswapRetriever, Cleanup, error) {
	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()

	kdht, err := initDHT(ctx, libp2p)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot init dht")
	}

	//nolint:gomnd
	bstoreDatastore, err := flatfs.CreateOrOpen(filepath.Join(dir, "blockstore"), flatfs.NextToLast(3), false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create or open flatfs blockstore")
	}

	bstore := blockstore.NewBlockstoreNoPrefix(bstoreDatastore)

	datastore, err := levelds.NewDatastore(filepath.Join(dir, "datastore"), nil)
	if err != nil {
		bstoreDatastore.Close()
		return nil, nil, errors.Wrap(err, "cannot create leveldb datastore")
	}

	closer := func() {
		bstoreDatastore.Close()
		datastore.Close()
	}

	baseDisc := routing.NewRoutingDiscovery(kdht)

	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create libp2p host")
	}

	network := bsnet.NewFromIpfsHost(libp2p, baseDisc)
	bitswap := bitswap.New(ctx, network, bstore)

	return &BitswapRetriever{
			log:     log.With().Str("role", "retrieval_bitswap").Caller().Logger(),
			libp2p:  libp2p,
			bs:      bstore,
			bitswap: bitswap,
			tmpDir:  dir,
		}, func() {
			os.RemoveAll(dir)
			libp2p.Close()
			bitswap.Close()
			closer()
		}, nil
}

func (b *BitswapRetriever) Type() task.Type {
	return task.Retrieval
}

func (b *BitswapRetriever) Retreive(ctx context.Context, input module.ValidationInput, cid cid.Cid) (*module.ValidationResult, error) {
	queryContext, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	block, err := b.bitswap.GetBlock(queryContext, cid)

	if err != nil {
		return nil, errors.Wrap(err, "failed to get block")
	}

	jsonb, err := module.NewJSONB(block)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return &module.ValidationResult{
		ValidationInput: input,
		Result:          jsonb,
	}, nil
}
