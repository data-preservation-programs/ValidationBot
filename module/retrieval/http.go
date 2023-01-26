package retrieval

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"validation-bot/module"

	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/node/repo"
	"github.com/labstack/gommon/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/urfave/cli"
	"golang.org/x/exp/slices"
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

type HttpRetreiver struct {
	log    *zerolog.Logger
	libp2p host.Host
	client *http.Client
}

type httpRetrieverBuilder struct{}

func (h *httpRetrieverBuilder) Build(ctx context.Context, libp2p host.Host) (*HttpRetreiver, error) {
	log := zerolog.Ctx(ctx).With().Str("component", "httpRetriever").Logger()
	client := *http.DefaultClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &HttpRetreiver{
		log:    &log,
		libp2p: libp2p,
		client: &client,
	}, nil
}

type GetStorageMinerOptions struct {
	PreferHttp bool
}

type GetStorageMinerOption func(*GetStorageMinerOptions)

func GetRawAPIMulti(ctx *cli.Context, t repo.RepoType, version string) ([]HttpHead, error) {

	var httpHeads []HttpHead
	ainfos, err := GetAPIInfoMulti(ctx, t)
	if err != nil || len(ainfos) == 0 {
		return httpHeads, xerrors.Errorf("could not get API info for %s: %w", t.Type(), err)
	}

	for _, ainfo := range ainfos {
		addr, err := ainfo.DialArgs(version)
		if err != nil {
			return httpHeads, xerrors.Errorf("could not get DialArgs: %w", err)
		}
		httpHeads = append(httpHeads, HttpHead{addr: addr, header: ainfo.AuthHeader()})
	}

	if IsVeryVerbose {
		_, _ = fmt.Fprintf(ctx.App.Writer, "using raw API %s endpoint: %s\n", version, httpHeads[0].addr)
	}

	return httpHeads, nil
}

func GetRawAPI(ctx *cli.Context, t repo.RepoType, version string) (string, http.Header, error) {
	heads, err := GetRawAPIMulti(ctx, t, version)
	if err != nil {
		return "", nil, err
	}

	if len(heads) > 1 {
		log.Warnf("More than 1 header received when expecting only one")
	}

	return heads[0].addr, heads[0].header, nil
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

func (h *HttpRetreiver) Retreive(ctx context.Context, minerInfo *module.MinerInfoResult, pcid PieceCid) (*ResultContent, error) {
	protocols, err := minerSupporttedProtocols(context.Background(), *minerInfo.PeerID, h.libp2p)
	if err != nil {
		return nil, errors.Wrap(err, "error getting miner supported protocols")
	}

	isSupported := false
	var protocol types.Protocol

	for _, proto := range protocols.Protocols {
		if slices.Contains([]string{string(Http), string(Https)}, proto.Name) {
			protocol = proto
			isSupported = true
		}
	}

	if isSupported {
		for i, ma := range protocol.Addresses {
			url := multiaddrToNative(protocol.Name, ma)

			resp, err := h.client.Get(url)
			if err != nil {
				return nil, errors.Wrap(err, "error getting data from miner")
			}

			if resp.StatusCode != http.StatusOK && i == len(protocol.Addresses)-1 {
				return &ResultContent{
					Protocol:     Protocol(protocol.Name),
					Status:       QueryFailure,
					ErrorMessage: fmt.Sprintf("miner %s supports %s, but get request failed.", minerInfo.PeerID, protocol.Name),
				}, errors.New(fmt.Sprintf("miner %s supports %s, but get request failed.", minerInfo.PeerID, protocol.Name))
			}

			// continue retreiving
			return &ResultContent{
				Protocol:        Protocol(protocol.Name),
				Status:          Success,
				ErrorMessage:    "",
				CalculatedStats: CalculatedStats{
					// TODO
				},
			}, nil
		}
	} else {
		return &ResultContent{
			Protocol:     Protocol(protocol.Name),
			Status:       ProtocolUnsupported,
			ErrorMessage: fmt.Sprintf("miner %s does not support protocol", minerInfo.PeerID, protocol.Name),
		}, errors.New(fmt.Sprintf("miner %s does not support protocol", minerInfo.PeerID, protocol.Name))
	}
}
