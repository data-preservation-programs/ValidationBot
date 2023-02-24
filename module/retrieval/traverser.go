package retrieval

import (
	"bytes"
	"context"
	"io"

	cid "github.com/ipfs/go-cid"
	gocar "github.com/ipld/go-car"
	util "github.com/ipld/go-car/util"
	dagpb "github.com/ipld/go-codec-dagpb"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/pkg/errors"
)

// Block is all information and metadata about a block that is part of a car file.
type Block struct {
	BlockCID cid.Cid
	Data     []byte
	Offset   uint64
	Size     uint64
}

// OnNewCarBlockFunc is called during traveral when a new unique block is encountered.
type onNewBlockFunc func(Block)

type DataReader interface {
	Get(context.Context, cid.Cid) (Block, error)
}

type Traverser struct {
	dags       []gocar.Dag
	reader     DataReader
	onNewBlock onNewBlockFunc
	offset     uint64
	size       uint64
	cidSet     *cid.Set
	cids       []cid.Cid
	lsys       ipld.LinkSystem
}

func NewTraverser(bitswap *BitswapRetriever, dags []gocar.Dag) (*Traverser, error) {
	var cids []cid.Cid

	onNewBlock := func(block Block) {
		cids = append(cids, block.BlockCID)

		bitswap.onNewBlock(block)
	}

	traverser := &Traverser{
		dags,
		bitswap,
		onNewBlock,
		0,
		0,
		cid.NewSet(),
		cids,
		cidlink.DefaultLinkSystem(),
	}

	traverser.lsys.StorageReadOpener = traverser.loader

	return traverser, nil
}

// Size returns the total size in bytes of the car file that will be written.
func (t Traverser) Size() uint64 {
	return t.size
}

func (t *Traverser) loader(ctx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
	cl, ok := lnk.(cidlink.Link)
	if !ok {
		return nil, errors.New("incorrect link type")
	}
	c := cl.Cid
	data, err := t.reader.Get(ctx.Ctx, cl.Cid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block from loader")
	}

	if !t.cidSet.Has(c) {
		t.cidSet.Add(c)
		size := util.LdSize(c.Bytes(), data)
		t.onNewBlock(Block{
			BlockCID: c,
			Data:     data,
			Offset:   t.offset,
			Size:     size,
		})
		t.offset += size
		t.size += size
	}
	return bytes.NewReader(data), nil
}

func (t Traverser) traverse(context context.Context) error {
	err := t.traverseBlocks(context)
	if errors.Is(err, ErrMaxTimeReached) {
		return ErrMaxTimeReached
	}
	if err != nil {
		return errors.Wrap(err, "failed to traverse blocks")
	}
	return nil
}

func (t *Traverser) traverseBlocks(ctx context.Context) error {
	nsc := func(lnk ipld.Link, lctx ipld.LinkContext) (ipld.NodePrototype, error) {
		// We can decode all nodes into basicnode's Any, except for
		// dagpb nodes, which must explicitly use the PBNode prototype.
		if lnk, ok := lnk.(cidlink.Link); ok && lnk.Cid.Prefix().Codec == 0x70 {
			return dagpb.Type.PBNode, nil
		}
		return basicnode.Prototype.Any, nil
	}

	for _, carDag := range t.dags {
		parsed, err := selector.ParseSelector(carDag.Selector)
		if err != nil {
			return errors.Wrap(err, "failed to parse selector")
		}
		lnk := cidlink.Link{Cid: carDag.Root}
		// nolint:exhaustruct
		ns, _ := nsc(lnk, ipld.LinkContext{}) // nsc won't error
		// nolint:exhaustruct,varnamelen
		nd, err := t.lsys.Load(ipld.LinkContext{Ctx: ctx}, lnk, ns)
		if err != nil {
			return errors.Wrap(err, "failed to load root node")
		}
		// nolint:exhaustruct
		prog := traversal.Progress{
			Cfg: &traversal.Config{
				Ctx:                            ctx,
				LinkSystem:                     t.lsys,
				LinkTargetNodePrototypeChooser: nsc,
				LinkVisitOnlyOnce:              true,
			},
		}
		err = prog.WalkAdv(nd, parsed, func(traversal.Progress, ipld.Node, traversal.VisitReason) error { return nil })
		if err != nil {
			return errors.Wrap(err, "failed to walk root node")
		}
	}
	return nil
}
