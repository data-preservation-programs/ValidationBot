package retrieval

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	"github.com/filecoin-project/lotus/api"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/jackc/pgtype"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"
)

type Dispatcher struct {
	dealResolver module.DealStatesResolver
}

func NewDispatcher(dealResolver module.DealStatesResolver) Dispatcher {
	return Dispatcher{
		dealResolver: dealResolver,
	}
}

func genRandNumber(max int) int {
	bg := big.NewInt(int64(max))

	n, err := rand.Int(rand.Reader, bg)
	if err != nil {
		panic(err)
	}

	return int(n.Int64())
}

func GetSupportedProtocols() []Protocol {
	return []Protocol{GraphSync, Bitswap}
}

func (Dispatcher) Type() task.Type {
	return task.Retrieval
}

func (d Dispatcher) GetTask(definition task.Definition) (*module.ValidationInput, error) {
	def := new(TaskDefinition)

	err := definition.Definition.AssignTo(def)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal definition")
	}

	var dataCid string
	var pieceCid string
	var label string
	var dealID string
	var client string

	switch {
	case len(def.DataCids) > 0:
		index := genRandNumber(len(def.DataCids))
		dataCid = def.DataCids[index]
	case len(def.PieceCids) > 0:
		index := genRandNumber(len(def.PieceCids))
		pieceCid = def.PieceCids[index]
	case len(def.FromClients) > 0:
		deals, err := d.dealResolver.DealsByProviderClients(definition.Target, def.FromClients)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get deals")
		}

		if len(deals) == 0 {
			log2.Warn().Str("role", "dispatcher").Str("moduleName", "retrieval").
				Str("provider", definition.Target).Msg("no deals found")
			return nil, nil
		}

		index := genRandNumber(len(deals))
		label = deals[index].Label
		pieceCid = deals[index].PieceCid
		dealID = deals[index].DealID
		client = deals[index].Client
	default:
		deals, err := d.dealResolver.DealsByProvider(definition.Target)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get deals")
		}

		if len(deals) == 0 {
			return nil, errors.New("no deals found")
		}

		index := genRandNumber(len(deals))
		label = deals[index].Label
		pieceCid = deals[index].PieceCid
		dealID = deals[index].DealID
		client = deals[index].Client
	}

	input := Input{
		ProtocolPreference: def.ProtocolPreference,
		DataCid:            dataCid,
		PieceCid:           pieceCid,
		Label:              label,
		DealID:             dealID,
		Client:             client,
	}

	jsonb, err := module.NewJSONB(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal validationInput")
	}

	validationInput := module.ValidationInput{
		Task: task.Task{
			Type:         definition.Type,
			DefinitionID: definition.ID,
			Target:       definition.Target,
			Tag:          definition.Tag,
			TaskID:       uuid.New(),
		},
		Input: jsonb,
	}

	return &validationInput, nil
}

func (Dispatcher) Validate(definition task.Definition) error {
	def := new(TaskDefinition)

	err := definition.Definition.AssignTo(def)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal definition")
	}

	if len(def.ProtocolPreference) >= 1 {
		for _, protocol := range def.ProtocolPreference {
			if !slices.Contains(GetSupportedProtocols(), protocol) {
				return errors.New(
					fmt.Sprintf(
						"protocol %s is not supported, currently only GraphSync and Bitswap protocol are supported",
						string(protocol),
					),
				)
			}
		}
	}

	if definition.IntervalSeconds > 0 && definition.IntervalSeconds < 3600 {
		return errors.New("interval must be at least 1 hour")
	}

	return nil
}

type Auditor struct {
	lotusAPI         api.Gateway
	log              zerolog.Logger
	timeout          time.Duration
	graphsync        GraphSyncRetrieverBuilder
	bitswap          BitswapRetrieverBuilder
	sem              *semaphore.Weighted
	locationFilter   module.LocationFilterConfig
	locationResolver module.IPInfoResolver
}

func (Auditor) Type() task.Type {
	return task.Retrieval
}

func NewAuditor(
	lotusAPI api.Gateway,
	graphsync GraphSyncRetrieverBuilder,
	bitswap BitswapRetrieverBuilder,
	timeout time.Duration,
	maxJobs int64,
	locationFilter module.LocationFilterConfig,
	locationResolver module.IPInfoResolver,
) (*Auditor, error) {
	return &Auditor{
		lotusAPI:         lotusAPI,
		log:              log2.With().Str("role", "retrieval_auditor").Caller().Logger(),
		timeout:          timeout,
		graphsync:        graphsync,
		bitswap:          bitswap,
		sem:              semaphore.NewWeighted(maxJobs),
		locationFilter:   locationFilter,
		locationResolver: locationResolver,
	}, nil
}

func (q Auditor) matchLocation(ctx context.Context, minerInfo *module.MinerInfoResult) bool {
	for _, addr := range minerInfo.MultiAddrs {
		countryCode, err := q.locationResolver.ResolveMultiAddr(ctx, addr)

		if err != nil {
			q.log.Error().Err(err).Msg("failed to resolve multiaddr with ipinfo.io")
			continue
		}

		continentCode := q.locationResolver.Continents[countryCode]

		q.log.Debug().Str("provider", minerInfo.MinerAddress.String()).
			Str("continent", continentCode).Str("country", countryCode).
			Msg("trying to match the provider with location filter")

		if q.locationFilter.Match(countryCode, continentCode) {
			return true
		}
	}

	return false
}

func (q Auditor) ShouldValidate(ctx context.Context, input module.ValidationInput) (bool, error) {
	provider := input.Target

	q.log.Debug().Str("provider", provider).Msg("retrieving miner info")

	minerInfoResult, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return false, errors.Wrap(err, "failed to get miner info")
	}

	// We still want to validate the provider so the error can be recorded
	if minerInfoResult.ErrorCode != "" || len(minerInfoResult.MultiAddrs) == 0 {
		return true, nil
	}

	if !q.matchLocation(ctx, minerInfoResult) {
		q.log.Info().Str("provider", provider).Msg("miner location does not match filter")
		return false, nil
	}

	return true, nil
}

//nolint:cyclop,funlen,maintidx,gocyclo
func (q Auditor) Validate(ctx context.Context, validationInput module.ValidationInput) (
	*module.ValidationResult,
	error,
) {
	if !q.sem.TryAcquire(1) {
		q.log.Debug().Msg("retrieval auditor has hit maximum jobs, waiting to acquire semaphore")

		err := q.sem.Acquire(ctx, 1)
		if err != nil {
			return nil, errors.Wrap(err, "failed to acquire semaphore")
		}
	}
	defer q.sem.Release(1)
	q.log.Info().Str("target", validationInput.Target).Msg("starting retrieval validation")
	provider := validationInput.Target

	input := new(Input)

	err := validationInput.Input.AssignTo(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal validationInput")
	}

	results := make(map[Protocol]ResultContent)

	totalBytes := uint64(0)
	minTTFB := time.Duration(0)
	maxAvgSpeed := float64(0)

	q.log.Debug().Str("provider", provider).Msg("retrieving miner info")

	minerInfoResult, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	// nolint:ineffassign
	lastStatus := ResultStatus("skipped")
	lastErrorMessage := ""

	var jsonb pgtype.JSONB

	switch {
	case minerInfoResult.ErrorCode != "":
		lastStatus = ResultStatus(minerInfoResult.ErrorCode)
		lastErrorMessage = minerInfoResult.ErrorMessage
	case len(minerInfoResult.MultiAddrs) == 0:
		lastStatus = module.NoMultiAddress
	case !q.matchLocation(ctx, minerInfoResult):
		q.log.Info().Str("provider", provider).Msg("miner location does not match filter")
		return nil, nil
	default:
		for _, protocol := range input.ProtocolPreference {
			q.log.Info().Str("provider", provider).Str("protocol", string(protocol)).Msg("starting retrieval")

			dataCidOrLabel := input.DataCid
			if dataCidOrLabel == "" {
				dataCidOrLabel = input.Label
			}

			if dataCidOrLabel == "" {
				q.log.Error().Str("provider", provider).Str(
					"protocol",
					string(protocol),
				).Msg("dataCid or label is required")
				continue
			}

			dataCid, err := cid.Decode(dataCidOrLabel)
			if err != nil {
				q.log.Error().Err(err).Str("provider", provider).Str(
					"protocol",
					string(protocol),
				).Msg("failed to decode data cid")
				continue
			}

			// nolint:exhaustive
			switch protocol {
			case GraphSync:
				retriever, cleanup, err := q.graphsync.Build()
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Str(
						"protocol",
						string(protocol),
					).Msg("failed to build GraphSync retriever")
					cleanup()
					continue
				}

				result, err := retriever.Retrieve(ctx, minerInfoResult.MinerAddress, dataCid, q.timeout)
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Str(
						"protocol",
						string(protocol),
					).Msg("failed to retrieve data with GraphSync protocol")

					cleanup()
					continue
				}

				cleanup()

				result.Protocol = GraphSync
				results[GraphSync] = *result
				lastStatus = result.Status
				lastErrorMessage = result.ErrorMessage

				totalBytes += result.BytesDownloaded

				if minTTFB == 0 || result.TimeToFirstByte < minTTFB {
					minTTFB = result.TimeToFirstByte
				}

				if result.AverageSpeedPerSec > maxAvgSpeed {
					maxAvgSpeed = result.AverageSpeedPerSec
				}

				if result.Status == Success {
					q.log.Info().Str("provider", provider).Str("protocol", string(protocol)).Msg("retrieval succeeded")
				}
			case Bitswap:
				libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Str(
						"protocol",
						string(protocol),
					).Msg("failed to create libp2p host")
				}

				protoprovider := NewProtocolProvider(libp2p)

				protocols, err := protoprovider.GetMinerProtocols(ctx, peer.AddrInfo{
					ID:    *minerInfoResult.PeerID,
					Addrs: minerInfoResult.MultiAddrs,
				})
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Str(
						"protocol",
						string(protocol),
					).Msg("failed to get miner protocols")
					continue
				}

				var protocol MinerProtocols

				for _, p := range protocols {
					if p.Protocol.Name == string(Bitswap) {
						protocol = p
					}
				}

				if protocol.Protocol.Name == "" {
					q.log.Error().Err(err).Str("provider", provider).Msg(
						"miner does not support Bitswap protocol")
					continue
				}

				// nolint:contextcheck
				bitswap, cleanup, err := q.bitswap.Build(minerInfoResult, protocol, libp2p)
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Str(
						"protocol",
						protocol.MultiAddrStr[0],
					).Msg("failed to build Bitswap retriever")
					cleanup()
					continue
				}

				result, err := bitswap.Retrieve(ctx, dataCid, q.timeout)
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Msg(
						fmt.Sprintf("failed to retrieve data with Bitswap protocol: %v", protocol.MultiAddrStr))
					cleanup()
					continue
				}

				cleanup()
				result.Protocol = Bitswap
				results[Bitswap] = *result
				lastStatus = result.Status
				lastErrorMessage = result.ErrorMessage
				totalBytes += result.BytesDownloaded

				if minTTFB == 0 || result.TimeToFirstByte < minTTFB {
					minTTFB = result.TimeToFirstByte
				}

				if result.AverageSpeedPerSec > maxAvgSpeed {
					maxAvgSpeed = result.AverageSpeedPerSec
				}

				if result.Status == Success {
					for _, addr := range protocol.MultiAddrStr {
						q.log.Info().Str("provider", provider).Str("protocol", addr).Msg("retrieval succeeded")
					}
				}
			default:
				return nil, errors.Errorf("unsupported protocol: %s", protocol)
			}
		}
	}

	result := Result{
		Status:                lastStatus,
		ErrorMessage:          lastErrorMessage,
		TotalBytesDownloaded:  totalBytes,
		MaxAverageSpeedPerSec: maxAvgSpeed,
		MinTimeToFirstByte:    minTTFB,
		Results:               results,
	}

	jsonb, err = module.NewJSONB(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return &module.ValidationResult{
		ValidationInput: validationInput,
		Result:          jsonb,
	}, nil
}
