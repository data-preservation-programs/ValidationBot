package retrieval

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"validation-bot/module"
	"validation-bot/task"

	"github.com/filecoin-project/lotus/api"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
)

type Dispatcher struct {
	minInterval  time.Duration
	lastRun      map[string]time.Time
	dealResolver module.DealStatesResolver
}

func NewDispatcher(minInterval time.Duration, dealResolver module.DealStatesResolver) Dispatcher {
	return Dispatcher{
		minInterval:  minInterval,
		lastRun:      make(map[string]time.Time),
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

func (d Dispatcher) GetTasks(definitions []task.Definition) (map[uuid.UUID]module.ValidationInput, error) {
	// Group by provider
	definitionsByProvider := make(map[string][]task.Definition)
	for _, def := range definitions {
		definitionsByProvider[def.Target] = append(definitionsByProvider[def.Target], def)
	}

	// Choose a random definition for each provider
	inputs := make(map[uuid.UUID]module.ValidationInput)

	for _, defs := range definitionsByProvider {
		lastRun, ok := d.lastRun[defs[0].Target]
		if !ok || time.Since(lastRun) > d.minInterval {
			index := genRandNumber(len(defs))
			def := defs[index]

			input, err := d.GetTask(def)
			if err != nil {
				log2.Error().Str("role", "dispatcher").
					Str("moduleName", "retrieval").Err(err).Msg("failed to get task")
				continue
			}

			if input != nil {
				inputs[input.Task.DefinitionID] = *input
				d.lastRun[def.Target] = time.Now()
			}
		}
	}

	return inputs, nil
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

	if len(def.ProtocolPreference) != 1 || def.ProtocolPreference[0] != GraphSync {
		return errors.New("currently only GraphSync protocol is supported")
	}

	return nil
}

type Auditor struct {
	lotusAPI         api.Gateway
	log              zerolog.Logger
	timeout          time.Duration
	graphsync        GraphSyncRetrieverBuilder
	sem              *semaphore.Weighted
	locationFilter   module.LocationFilterConfig
	locationResolver *module.GeoLite2Resolver
}

func NewAuditor(
	lotusAPI api.Gateway,
	graphsync GraphSyncRetrieverBuilder,
	timeout time.Duration,
	maxJobs int64,
	locationFilter module.LocationFilterConfig,
) (*Auditor, error) {
	geolite2Resolver, err := module.NewGeoLite2Resolver()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create geolite2 resolver")
	}

	return &Auditor{
		lotusAPI:         lotusAPI,
		log:              log2.With().Str("role", "retrieval_auditor").Caller().Logger(),
		timeout:          timeout,
		graphsync:        graphsync,
		sem:              semaphore.NewWeighted(maxJobs),
		locationFilter:   locationFilter,
		locationResolver: geolite2Resolver,
	}, nil
}

func (q Auditor) matchLocation(minerInfo *module.MinerInfoResult) bool {
	for _, addr := range minerInfo.MultiAddrs {
		city, err := q.locationResolver.ResolveMultiAddr(addr)
		if err != nil {
			q.log.Error().Err(err).Msg("failed to resolve multiaddr with geolite2")
			continue
		}

		q.log.Debug().Str("provider", minerInfo.MinerAddress.String()).
			Str("continent", city.Continent.Code).Str("country", city.Country.IsoCode).
			Msg("trying to match the provider with location filter")

		if q.locationFilter.Match(city) {
			return true
		}
	}

	return false
}

//nolint:cyclop
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

	results := make(map[string]ResultContent)
	totalBytes := uint64(0)
	minTTFB := time.Duration(0)
	maxAvgSpeed := float64(0)

	q.log.Debug().Str("provider", provider).Msg("retrieving miner info")

	minerInfoResult, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	lastStatus := ResultStatus("skipped")
	lastErrorMessage := ""

	switch {
	case minerInfoResult.ErrorCode != "":
		lastStatus = ResultStatus(minerInfoResult.ErrorCode)
		lastErrorMessage = minerInfoResult.ErrorMessage
	case len(minerInfoResult.MultiAddrs) == 0:
		lastStatus = module.NoMultiAddress
	case !q.matchLocation(minerInfoResult):
		q.log.Info().Str("provider", provider).Msg("miner location does not match filter")
		return nil, nil
	default:
		for _, protocol := range input.ProtocolPreference {
			q.log.Info().Str("provider", provider).Str("protocol", string(protocol)).Msg("starting retrieval")

			dataCidOrLabel := input.DataCid
			if dataCidOrLabel == "" {
				dataCidOrLabel = input.Label
			}

			switch protocol {
			case GraphSync:
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
					).Msg("failed to decode data cid for GraphSync protocol")
					continue
				}

				retriever, cleanup, err := q.graphsync.Build()
				if err != nil {
					q.log.Error().Err(err).Str("provider", provider).Str(
						"protocol",
						string(protocol),
					).Msg("failed to build GraphSync retriever")
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
				results[string(GraphSync)] = *result
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
					break
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

	jsonb, err := module.NewJSONB(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return &module.ValidationResult{
		ValidationInput: validationInput,
		Result:          jsonb,
	}, nil
}
