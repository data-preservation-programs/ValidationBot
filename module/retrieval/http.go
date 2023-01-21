package retrieval

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"path"
	"sort"
	"strings"

	"github.com/filecoin-project/boost/cmd"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	multiaddrutil "github.com/filecoin-project/go-legs/httpsync/multiaddr"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/node/repo"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/urfave/cli"
	"golang.org/x/xerrors"
)

type httpTransport struct {
	libp2pHost   host.Host
	libp2pClient *http.Client

	minBackOffWait       time.Duration
	maxBackoffWait       time.Duration
	backOffFactor        float64
	maxReconnectAttempts float64
}

type httpRetreiver struct {
	log          *zerolog.Logger
	libp2p       host.Host
	libp2pClient *http.Client
}

type httpRetrieverBuilder struct{}

func (h *httpRetrieverBuilder) Build(ctx context.Context, libp2p host.Host) (Retriever, error) {
	log := zerolog.Ctx(ctx).With().Str("component", "httpRetriever").Logger()
	client, err := http.Client{
		Timeout: 30 * time.Second,
	}

	if err != nil {
		return nil, err
	}

	return &httpRetreiver{
		log:          log,
		libp2p:       libp2p,
		libp2pClient: client,
	}, nil
}

func GetStorageMinerAPI(ctx *cli.Context, opts ...GetStorageMinerOption) (api.StorageMiner, jsonrpc.ClientCloser, error) {
	var options GetStorageMinerOptions
	for _, opt := range opts {
		opt(&options)
	}

	if tn, ok := ctx.App.Metadata["testnode-storage"]; ok {
		return tn.(api.StorageMiner), func() {}, nil
	}

	addr, headers, err := GetRawAPI(ctx, repo.StorageMiner, "v0")
	if err != nil {
		return nil, nil, err
	}

	if options.PreferHttp {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, nil, xerrors.Errorf("parsing miner api URL: %w", err)
		}

		switch u.Scheme {
		case "ws":
			u.Scheme = "http"
		case "wss":
			u.Scheme = "https"
		}

		addr = u.String()
	}

	if IsVeryVerbose {
		_, _ = fmt.Fprintln(ctx.App.Writer, "using miner API v0 endpoint:", addr)
	}

	return client.NewStorageMinerRPCV0(ctx.Context, addr, headers)
}

func multiaddrToNative(proto string, ma multiaddr.Multiaddr) string {
	switch proto {
	case "http", "https":
		u, err := multiaddrutil.ToURL(ma)
		if err != nil {
			return ""
		}
		return u.String()
	}

	return ""
}

func getPushUrl(addr string) (string, error) {
	pushUrl, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	switch pushUrl.Scheme {
	case "ws":
		pushUrl.Scheme = "http"
	case "wss":
		pushUrl.Scheme = "https"
	}
	///rpc/v0 -> /rpc/streams/v0/push

	pushUrl.Path = path.Join(pushUrl.Path, "../streams/v0/push")
	return pushUrl.String(), nil
}

func getMinerProtocols(ctx context.Context, api api.FullNode, miner address.Address) ([]string, error) {
	api, closer, err := lcli.GetGatewayAPI(ctx)
	if err != nil {
		return nil, fmt.Errorf("setting up gateway connection: %w", err)
	}
	defer closer()

	addrStr := ctx.Args().Get(0)
	maddr, err := address.NewFromString(addrStr)
	if err != nil {
		return nil, fmt.Errorf("parsing provider on-chain address %s: %w", addrStr, err)
	}

	addrInfo, err := cmd.GetAddrInfo(ctx, api, maddr)
	if err != nil {
		return nil, fmt.Errorf("getting provider multi-address: %w", err)
	}

	// log.Debugw("connecting to storage provider",
	// 	"id", addrInfo.ID, "multiaddrs", addrInfo.Addrs, "addr", maddr)

	if err := n.Host.Connect(ctx, *addrInfo); err != nil {
		return nil, fmt.Errorf("connecting to peer %s: %w", addrInfo.ID, err)
	}

	protos, err := n.Host.Peerstore().GetProtocols(addrInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("getting protocols for peer %s: %w", addrInfo.ID, err)
	}
	sort.Strings(protos)

	agentVersionI, err := n.Host.Peerstore().Get(addrInfo.ID, "AgentVersion")
	if err != nil {
		return nil, fmt.Errorf("getting agent version for peer %s: %w", addrInfo.ID, err)
	}
	agentVersion, _ := agentVersionI.(string)

	if ctx.Bool("json") {
		return cmd.PrintJson(map[string]interface{}{
			"provider":   addrStr,
			"agent":      agentVersion,
			"id":         addrInfo.ID.String(),
			"multiaddrs": addrInfo.Addrs,
			"protocols":  protos,
		})
	}

	fmt.Println("Provider: " + addrStr)
	fmt.Println("Agent: " + agentVersion)
	fmt.Println("Peer ID: " + addrInfo.ID.String())
	fmt.Println("Peer Addresses:")
	for _, addr := range addrInfo.Addrs {
		fmt.Println("  " + addr.String())
	}
	fmt.Println("Protocols:\n" + "  " + strings.Join(protos, "\n  "))
	return nil, nil
}
