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
	minInterval time.Duration
	lastRun     map[string]time.Time
}

func NewDispatcher(minInterval time.Duration) Dispatcher {
	return Dispatcher{
		minInterval: minInterval,
		lastRun:     make(map[string]time.Time),
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
				return nil, errors.Wrap(err, "failed to get task")
			}

			inputs[input.Task.DefinitionID] = input
			d.lastRun[def.Target] = time.Now()
		}
	}

	return inputs, nil
}

func (d Dispatcher) GetTask(definition task.Definition) (module.ValidationInput, error) {
	def := new(TaskDefinition)

	err := definition.Definition.AssignTo(def)
	if err != nil {
		return module.ValidationInput{}, errors.Wrap(err, "failed to unmarshal definition")
	}

	var dataCid string
	var pieceCid string

	switch {
	case len(def.DataCids) > 0:
		index := genRandNumber(len(def.DataCids))
		dataCid = def.DataCids[index]
	case len(def.PieceCids) > 0:
		index := genRandNumber(len(def.PieceCids))
		pieceCid = def.PieceCids[index]
	default:
		return module.ValidationInput{}, errors.New("no data or piece cids specified")
	}

	input := Input{
		ProtocolPreference: def.ProtocolPreference,
		DataCid:            dataCid,
		PieceCid:           pieceCid,
	}

	jsonb, err := module.NewJSONB(input)
	if err != nil {
		return module.ValidationInput{}, errors.Wrap(err, "failed to marshal validationInput")
	}

	validationInput := module.ValidationInput{
		Task: task.Task{
			Type:         definition.Type,
			DefinitionID: definition.ID,
			Target:       definition.Target,
		},
		Input: jsonb,
	}

	return validationInput, nil
}

func (Dispatcher) Validate(definition task.Definition) error {
	def := new(TaskDefinition)

	err := definition.Definition.AssignTo(def)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal definition")
	}

	if len(def.DataCids) == 0 && len(def.PieceCids) == 0 {
		return errors.New("no data or piece cids specified")
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

		if q.locationFilter.Match(city) {
			return true
		}
	}

	return false
}

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

	results := make([]ResultContent, 0, len(input.ProtocolPreference))
	totalBytes := uint64(0)
	minTTFB := time.Duration(0)
	maxAvgSpeed := float64(0)
	auditorErrors := make([]string, 0)

	q.log.Debug().Str("provider", provider).Msg("retrieving miner info")

	minerInfoResult, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	//nolint:nestif
	if minerInfoResult.ErrorCode != "" {
		results = append(
			results, ResultContent{
				Status:       ResultStatus(minerInfoResult.ErrorCode),
				ErrorMessage: minerInfoResult.ErrorMessage,
			},
		)
	} else {
		if !q.matchLocation(minerInfoResult) {
			q.log.Info().Str("provider", provider).Msg("miner location does not match filter")
			return nil, nil
		}

		for _, protocol := range input.ProtocolPreference {
			q.log.Info().Str("provider", provider).Str("protocol", string(protocol)).Msg("starting retrieval")

			switch protocol {
			case GraphSync:
				if input.DataCid == "" {
					auditorErrors = append(auditorErrors, "data cid is required for GraphSync protocol")
					continue
				}

				dataCid, err := cid.Decode(input.DataCid)
				if err != nil {
					auditorErrors = append(
						auditorErrors,
						"failed to decode data cid for GraphSync protocol: "+err.Error(),
					)
					continue
				}

				retriever, cleanup, err := q.graphsync.Build()
				if err != nil {
					auditorErrors = append(auditorErrors, "failed to build GraphSync retriever: "+err.Error())
					continue
				}

				result, err := retriever.Retrieve(ctx, minerInfoResult.MinerAddress, dataCid, q.timeout)
				if err != nil {
					auditorErrors = append(
						auditorErrors,
						"failed to retrieve data with GraphSync protocol: "+err.Error(),
					)

					cleanup()
					continue
				}

				cleanup()

				result.Protocol = GraphSync
				results = append(results, *result)
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
		AuditorErrors:         auditorErrors,
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
		Task:   validationInput.Task,
		Result: jsonb,
	}, nil
}
