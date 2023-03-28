package retrieval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"validation-bot/module"
	mock2 "validation-bot/module/mock"
	"validation-bot/task"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRetrieval_genRandNumber(t *testing.T) {
	assert := assert.New(t)
	a := genRandNumber(10)
	assert.GreaterOrEqual(a, 0)
	assert.Less(a, 10)
}

func TestRetrieval_GetTask_DataCidsProvided(t *testing.T) {
	assert := assert.New(t)
	def := TaskDefinition{
		ProtocolPreference: []Protocol{GraphSync},
		DataCids:           []string{"cid1", "cid2", "cid3", "cid4"},
		PieceCids:          []string{"cid5", "cid6"},
	}
	definition, err := module.NewJSONB(def)
	assert.NoError(err)
	taskDef := task.Definition{
		Target:     "provider",
		Definition: definition,
		ID:         uuid.New(),
		Type:       task.Retrieval,
	}
	dispatcher := NewDispatcher(new(mock2.MockDealStatesResolver))
	input, err := dispatcher.GetTask(taskDef)
	assert.NoError(err)
	assert.Equal(taskDef.ID, input.DefinitionID)
	assert.Equal(taskDef.Target, input.Target)
	assert.Equal(taskDef.Type, input.Type)
	assert.Regexp(
		`{"protocolPreference":\["GraphSync"\],"dataCid":"cid[1234]"}`,
		string(input.Input.Bytes),
	)
}

func TestRetrieval_GetTask_PieceCidsProvided(t *testing.T) {
	assert := assert.New(t)
	def := TaskDefinition{
		ProtocolPreference: []Protocol{GraphSync},
		DataCids:           []string{},
		PieceCids:          []string{"cid1", "cid2", "cid3", "cid4"},
	}
	definition, err := module.NewJSONB(def)
	assert.NoError(err)
	taskDef := task.Definition{
		Target:     "provider",
		Definition: definition,
		ID:         uuid.New(),
		Type:       task.Retrieval,
	}
	dispatcher := NewDispatcher(new(mock2.MockDealStatesResolver))
	input, err := dispatcher.GetTask(taskDef)
	assert.NoError(err)
	assert.Equal(taskDef.ID, input.DefinitionID)
	assert.Equal(taskDef.Target, input.Target)
	assert.Equal(taskDef.Type, input.Type)
	assert.Regexp(
		`{"protocolPreference":\["GraphSync"\],"pieceCid":"cid[1234]"}`,
		string(input.Input.Bytes),
	)
}

func TestRetrieval_GetTask_ClientIdProvided(t *testing.T) {
	assert := assert.New(t)
	def := TaskDefinition{
		ProtocolPreference: []Protocol{GraphSync},
		DataCids:           []string{},
		PieceCids:          []string{},
		FromClients:        []string{"client1"},
	}
	definition, err := module.NewJSONB(def)
	assert.NoError(err)
	taskDef := task.Definition{
		Target:     "provider",
		Definition: definition,
		ID:         uuid.New(),
		Type:       task.Retrieval,
	}
	resolver := new(mock2.MockDealStatesResolver)
	dispatcher := NewDispatcher(resolver)
	resolver.On("DealsByProviderClients", "provider", []string{"client1"}).Return(
		[]module.DealStateModel{
			{
				Label:    "label1",
				PieceCid: "piece1",
				DealID:   "deal1",
				Client:   "client1",
			},
			{
				Label:    "label2",
				PieceCid: "piece2",
				DealID:   "deal2",
				Client:   "client2",
			},
		}, nil,
	)
	task, err := dispatcher.GetTask(taskDef)
	assert.NoError(err)
	assert.Contains(string(task.Input.Bytes), `label":"label`)
	assert.Contains(string(task.Input.Bytes), `dealId":"deal`)
	assert.Contains(string(task.Input.Bytes), `client":"client`)
}

func TestRetrieval_Dispatcher_Validate_InvalidProtocol(t *testing.T) {
	assert := assert.New(t)
	def := TaskDefinition{
		ProtocolPreference: []Protocol{Protocol("invalid")},
		DataCids:           []string{"cid1", "cid2", "cid3", "cid4"},
		PieceCids:          []string{"cid5", "cid6"},
	}
	definition, err := module.NewJSONB(def)
	assert.NoError(err)
	taskDef := task.Definition{
		Target:     "provider",
		Definition: definition,
		ID:         uuid.New(),
		Type:       task.Retrieval,
	}
	dispatcher := NewDispatcher(new(mock2.MockDealStatesResolver))
	err = dispatcher.Validate(taskDef)
	assert.ErrorContains(err, "currently only GraphSync and Bitswap protocol are supported")
}

func TestRetrieval_Dispatcher_Validate_IntervalTooShort(t *testing.T) {
	assert := assert.New(t)

	cases := []Protocol{GraphSync, Bitswap}
	for _, protocol := range cases {
		def := TaskDefinition{
			ProtocolPreference: []Protocol{Protocol(protocol)},
			DataCids:           []string{"cid1", "cid2", "cid3", "cid4"},
			PieceCids:          []string{"cid5", "cid6"},
		}
		definition, err := module.NewJSONB(def)
		assert.NoError(err)
		taskDef := task.Definition{
			Target:          "provider",
			Definition:      definition,
			ID:              uuid.New(),
			Type:            task.Retrieval,
			IntervalSeconds: 1800,
		}
		dispatcher := NewDispatcher(new(mock2.MockDealStatesResolver))
		err = dispatcher.Validate(taskDef)
		assert.ErrorContains(err, "interval must be at least 1 hour")
	}
}

func TestRetrieval_CidNotGiven(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	defer closer()
	assert.NoError(err)

	graphsync := GraphSyncRetrieverBuilderImpl{
		LotusAPI: api,
		BaseDir:  "/tmp",
	}
	auditor, err := NewAuditor(api, graphsync, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{}, module.IPInfoResolver{})
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{GraphSync},
	}

	input, err := module.NewJSONB(in)
	assert.NoError(err)
	result, err := auditor.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f03223",
			},
			Input: input,
		},
	)

	assert.NoError(err)
	out := new(Result)
	err = result.Result.AssignTo(out)
	assert.NoError(err)
	fmt.Printf("%+v\n", out)
	assert.Equal(0, len(out.Results))
	assert.Equal("skipped", string(out.Status))
}

func TestRetrieval_DataNotFound(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	defer closer()
	assert.NoError(err)

	graphsync := GraphSyncRetrieverBuilderImpl{
		LotusAPI: api,
		BaseDir:  "/tmp",
	}
	auditor, err := NewAuditor(api, graphsync, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{}, module.IPInfoResolver{})
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{GraphSync},
		DataCid:            "bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa",
	}
	input, err := module.NewJSONB(in)
	assert.NoError(err)
	result, err := auditor.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f03223",
			},
			Input: input,
		},
	)
	assert.NoError(err)
	out := new(Result)
	err = result.Result.AssignTo(out)
	assert.NoError(err)
	fmt.Printf("%+v\n", out)
	assert.Equal(QueryResponseError, out.Results["GraphSync"].Status)
	assert.Equal(QueryResponseError, out.Status)
	assert.Contains(out.Results["GraphSync"].ErrorMessage, "key not found")
	assert.Contains(out.ErrorMessage, "key not found")
}

func TestAuditor_ShouldValidate_NoIfMinerNotMatchingLocationFilter(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	defer closer()
	assert.NoError(err)

	graphsync := GraphSyncRetrieverBuilderImpl{
		LotusAPI: api,
		BaseDir:  "/tmp",
	}
	auditor, err := NewAuditor(
		api, graphsync, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{
			Continent: []string{"AAA"},
		},
		module.IPInfoResolver{},
	)
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{GraphSync},
		DataCid:            "bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa",
	}
	input, err := module.NewJSONB(in)
	assert.NoError(err)
	result, err := auditor.ShouldValidate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f03223",
			},
			Input: input,
		},
	)
	assert.NoError(err)
	assert.False(result)
}

func TestAuditor_ShouldValidate_YesIfMinerDoesNotHaveMultiAddr(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	defer closer()
	assert.NoError(err)

	graphsync := GraphSyncRetrieverBuilderImpl{
		LotusAPI: api,
		BaseDir:  "/tmp",
	}
	auditor, err := NewAuditor(
		api, graphsync, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{
			Continent: []string{"AAA"},
		},
		module.IPInfoResolver{},
	)
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{GraphSync},
		DataCid:            "bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa",
	}
	input, err := module.NewJSONB(in)
	assert.NoError(err)
	result, err := auditor.ShouldValidate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f01173170",
			},
			Input: input,
		},
	)
	assert.NoError(err)
	assert.True(result)
}

func TestRetrieval_SkipIfMinerNotMatchingLocationFilter(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	defer closer()
	assert.NoError(err)

	graphsync := GraphSyncRetrieverBuilderImpl{
		LotusAPI: api,
		BaseDir:  "/tmp",
	}
	auditor, err := NewAuditor(
		api, graphsync, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{
			Continent: []string{"AAA"},
		},
		module.IPInfoResolver{},
	)
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{GraphSync},
		DataCid:            "bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa",
	}
	input, err := module.NewJSONB(in)
	assert.NoError(err)
	result, err := auditor.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f03223",
			},
			Input: input,
		},
	)
	assert.NoError(err)
	assert.Nil(result)
}

func TestRetrieval_SuccessMockGraphsyncRetrieval(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	assert.Nil(err)
	defer closer()
	mockRetriever := new(MockGraphSyncRetriever)
	mockRetrieverBuilder := MockGraphSyncRetrieverBuilder{Retriever: mockRetriever}
	auditor, err := NewAuditor(api, &mockRetrieverBuilder, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{}, module.IPInfoResolver{})
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{GraphSync},
		DataCid:            "bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa",
	}
	input, err := module.NewJSONB(in)
	assert.NoError(err)
	resultContent := &ResultContent{
		Status:   Success,
		Protocol: GraphSync,
		CalculatedStats: CalculatedStats{
			BytesDownloaded:    100,
			AverageSpeedPerSec: 10.0,
			TimeElapsed:        1 * time.Second,
			TimeToFirstByte:    2 * time.Second,
		},
	}
	mockRetriever.On("Retrieve", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(resultContent, nil)
	result, err := auditor.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f03223",
			},
			Input: input,
		},
	)
	assert.NoError(err)
	out := new(Result)
	err = result.Result.AssignTo(out)
	assert.NoError(err)
	fmt.Printf("%+v\n", out)
	assert.Equal(Success, out.Results["GraphSync"].Status)
	assert.Equal(Success, out.Status)
	assert.EqualValues(100, out.TotalBytesDownloaded)
	assert.EqualValues(10, out.MaxAverageSpeedPerSec)
	assert.Equal(2*time.Second, out.MinTimeToFirstByte)
}

func TestRetrieval_SuccessBitswapLiveRetrieval(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	assert.Nil(err)
	defer closer()
	mockRetriever := new(MockGraphSyncRetriever)
	mockRetrieverBuilder := MockGraphSyncRetrieverBuilder{Retriever: mockRetriever}
	auditor, err := NewAuditor(api, &mockRetrieverBuilder, BitswapRetrieverBuilder{}, 10*time.Second, 1, module.LocationFilterConfig{}, module.IPInfoResolver{})
	assert.NoError(err)
	in := Input{
		ProtocolPreference: []Protocol{Bitswap},
		DataCid:            "bafykbzaceb4gqljh5wrijjincngznnrw4f6hwjfpei4evvcwhhgh4jegjb4sy",
	}
	input, err := module.NewJSONB(in)
	assert.NoError(err)

	result, err := auditor.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f01953925",
			},
			Input: input,
		},
	)
	assert.NoError(err)
	out := new(Result)
	err = result.Result.AssignTo(out)
	assert.NoError(err)
	fmt.Printf("%+v\n", out)
	assert.Equal(Success, out.Results[Bitswap].Status)
	assert.Equal(Success, out.Status)
	assert.Greater(out.TotalBytesDownloaded, uint64(0))
	assert.Greater(out.MaxAverageSpeedPerSec, float64(0))
	assert.Greater(out.MinTimeToFirstByte, 0*time.Second)
}
