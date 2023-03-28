package indexprovider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multicodec"
)

type Publisher struct {
	rl     sync.RWMutex
	root   cid.Cid
	server *http.Server
}

func (p *Publisher) Serve(host host.Host, topic string) error {
	pid := protocol.ID(path.Join("/legs/head", topic, "0.0.1"))
	l, err := gostream.Listen(host, pid)
	if err != nil {
		return err
	}
	return p.server.Serve(l)
}

func (p *Publisher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	base := path.Base(r.URL.Path)
	if base != "head" {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	p.rl.RLock()
	defer p.rl.RUnlock()
	var out []byte
	if p.root != cid.Undef {
		currentHead := p.root.String()
		out = []byte(currentHead)
	} else {
	}

	_, err := w.Write(out)
	if err != nil {
	}
}

func (p *Publisher) UpdateRoot(_ context.Context, c cid.Cid) error {
	p.rl.Lock()
	defer p.rl.Unlock()
	p.root = c
	return nil
}

func (p *Publisher) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return p.server.Shutdown(ctx)
}

func NewPublisher() *Publisher {
	p := &Publisher{
		server: &http.Server{},
	}
	p.server.Handler = http.Handler(p)
	return p
}

func MkLinkSystem(ds datastore.Batching) ipld.LinkSystem {
	lsys := cidlink.DefaultLinkSystem()
	lsys.StorageReadOpener = func(_ ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		val, err := ds.Get(context.Background(), datastore.NewKey(lnk.String()))
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(val), nil
	}
	lsys.StorageWriteOpener = func(lctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		buf := bytes.NewBuffer(nil)
		return buf, func(lnk ipld.Link) error {
			return ds.Put(lctx.Ctx, datastore.NewKey(lnk.String()), buf.Bytes())
		}, nil
	}
	return lsys
}

func Store(srcStore datastore.Batching, n ipld.Node) (ipld.Link, error) {
	linkproto := cidlink.LinkPrototype{
		Prefix: cid.Prefix{
			Version:  1,
			Codec:    uint64(multicodec.DagJson),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: 16,
		},
	}
	lsys := MkLinkSystem(srcStore)

	return lsys.Store(ipld.LinkContext{}, linkproto, n)
}

func TestFetchLatestHead(t *testing.T) {
	const ipPrefix = "/ip4/127.0.0.1/tcp/"

	publisher, _ := libp2p.New()
	client, _ := libp2p.New()

	var addrs []multiaddr.Multiaddr
	for _, a := range publisher.Addrs() {
		// Change /ip4/127.0.0.1/tcp/<port> to /dns4/localhost/tcp/<port> to
		// test that dns address works.
		if strings.HasPrefix(a.String(), ipPrefix) {
			addrStr := "/dns4/localhost/tcp/" + a.String()[len(ipPrefix):]
			addr, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				t.Fatal(err)
			}
			addrs = append(addrs, addr)
			break
		}
	}

	// Provide multiaddrs to connect to
	client.Peerstore().AddAddrs(publisher.ID(), addrs, time.Hour)

	publisherStore := dssync.MutexWrap(datastore.NewMapDatastore())
	rootLnk, err := Store(publisherStore, basicnode.NewString("hello world"))
	if err != nil {
		t.Fatal(err)
	}

	p := NewPublisher()
	go p.Serve(publisher, "test")
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := QueryRootCid(ctx, client, "test", publisher.ID())
	if err != nil && c != cid.Undef {
		t.Fatal("Expected to get a nil error and a cid undef because there is no root")
	}

	if err := p.UpdateRoot(context.Background(), rootLnk.(cidlink.Link).Cid); err != nil {
		t.Fatal(err)
	}

	c, err = QueryRootCid(ctx, client, "test", publisher.ID())
	if err != nil {
		t.Fatal(err)
	}

	if !c.Equals(rootLnk.(cidlink.Link).Cid) {
		t.Fatalf("didn't get expected cid. expected %s, got %s", rootLnk, c)
	}
}
