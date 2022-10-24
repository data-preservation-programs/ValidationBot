package retrieval

import (
	"context"
	"math/rand"
	"time"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Dispatcher struct {
	MinInterval time.Duration
	LastRun     map[string]time.Time
}

func (d Dispatcher) GetTasks(definitions []task.Definition) (map[uuid.UUID]module.ValidationInput, error) {
	// Group by provider
	definitionsByProvider := make(map[string][]task.Definition)
	for _, def := range definitions {
		definitionsByProvider[def.Target] = append(definitionsByProvider[def.Target], def)
	}

	// Choose a random definition for each provider
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	inputs := make(map[uuid.UUID]module.ValidationInput)
	for _, defs := range definitionsByProvider {
		lastRun, ok := d.LastRun[defs[0].Target]
		if !ok || time.Since(lastRun) > d.MinInterval {
			index := r.Intn(len(defs))
			def := defs[index]
			input, err := d.GetTask(def)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get task")
			}
			inputs[input.Task.DefinitionID] = input
			d.LastRun[def.Target] = time.Now()
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

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	var dataCid string
	var pieceCid string
	if len(def.DataCids) > 0 {
		index := r.Intn(len(def.DataCids))
		dataCid = def.DataCids[index]
	} else if len(def.PieceCids) > 0 {
		index := r.Intn(len(def.PieceCids))
		pieceCid = def.PieceCids[index]
	} else {
		return module.ValidationInput{}, errors.New("no data or piece cids specified")
	}

	in := Input{
		ProtocolPreference: def.ProtocolPreference,
		DataCid:            dataCid,
		PieceCid:           pieceCid,
	}

	jsonb, err := module.NewJSONB(in)
	if err != nil {
		return module.ValidationInput{}, errors.Wrap(err, "failed to marshal input")
	}
	input := module.ValidationInput{
		Task: task.Task{
			Type:         definition.Type,
			DefinitionID: definition.ID,
			Target:       definition.Target,
		},
		Input: jsonb,
	}
	return input, nil
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
	log       zerolog.Logger
	timeout   time.Duration
	graphsync GraphSyncRetrieverBuilder
}

func NewAuditor(graphsync GraphSyncRetrieverBuilder, timeout time.Duration) Auditor {
	return Auditor{
		log:       log.With().Str("role", "retrieval_auditor").Logger(),
		timeout:   timeout,
		graphsync: graphsync,
	}
}

func (q Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	provider := input.Target
	in := new(Input)
	err := input.Input.AssignTo(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal input")
	}

	results := make([]ResultContent, 0, len(in.ProtocolPreference))
	totalBytes := uint64(0)
	minTTFB := time.Duration(0)
	maxAvgSpeed := float64(0)
	auditorErrors := make([]string, 0)
	for _, protocol := range in.ProtocolPreference {
		q.log.Info().Str("provider", provider).Str("protocol", string(protocol)).Msg("starting retrieval")

		switch protocol {
		case GraphSync:
			if in.DataCid == "" {
				auditorErrors = append(auditorErrors, "data cid is required for GraphSync protocol: "+err.Error())
				continue
			}
			dataCid, err := cid.Decode(in.DataCid)
			if err != nil {
				auditorErrors = append(auditorErrors, "failed to decode data cid for GraphSync protocol: "+err.Error())
				continue
			}
			retriever, cleanup, err := q.graphsync.Build()
			if err != nil {
				auditorErrors = append(auditorErrors, "failed to build GraphSync retriever: "+err.Error())
				continue
			}
			result, err := retriever.Retrieve(ctx, provider, dataCid, q.timeout)
			if err != nil {
				auditorErrors = append(auditorErrors, "failed to retrieve data with GraphSync protocol: "+err.Error())
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
		Task:   input.Task,
		Result: jsonb,
	}, nil
}