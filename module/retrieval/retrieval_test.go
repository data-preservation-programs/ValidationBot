package retrieval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"validation-bot/module"
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
	dispatcher := NewDispatcher(time.Second, new(module.MockDealStatesResolver))
	input, err := dispatcher.GetTask(taskDef)
	assert.NoError(err)
	assert.Equal(taskDef.ID, input.DefinitionID)
	assert.Equal(taskDef.Target, input.Target)
	assert.Equal(taskDef.Type, input.Type)
	assert.Regexp(
		`{"protocolPreference":\["GraphSync"\],"dataCid":"cid[1234]","pieceCid":""}`,
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
	dispatcher := NewDispatcher(time.Second, new(module.MockDealStatesResolver))
	input, err := dispatcher.GetTask(taskDef)
	assert.NoError(err)
	assert.Equal(taskDef.ID, input.DefinitionID)
	assert.Equal(taskDef.Target, input.Target)
	assert.Equal(taskDef.Type, input.Type)
	assert.Regexp(
		`{"protocolPreference":\["GraphSync"\],"dataCid":"","pieceCid":"cid[1234]"}`,
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
	resolver := new(module.MockDealStatesResolver)
	dispatcher := NewDispatcher(time.Second, resolver)
	resolver.On("DealsByProviderClients", "provider", []string{"client1"}).Return(
		[]module.SimplifiedDeal{
			{
				Label:    "label1",
				PieceCID: "piece1",
			},
			{
				Label:    "label2",
				PieceCID: "piece2",
			},
		}, nil,
	)
	task, err := dispatcher.GetTask(taskDef)
	assert.NoError(err)
	assert.Contains(string(task.Input.Bytes), `dataCid":"label`)
}

func TestRetrieval_GetTasks(t *testing.T) {
	assert := assert.New(t)
	def := TaskDefinition{
		ProtocolPreference: []Protocol{GraphSync},
		DataCids:           []string{"cid1", "cid2", "cid3", "cid4"},
		PieceCids:          []string{"cid5", "cid6"},
	}
	definition, err := module.NewJSONB(def)
	assert.NoError(err)
	taskDef1 := task.Definition{
		Target:     "provider1",
		Definition: definition,
		ID:         uuid.New(),
		Type:       task.Retrieval,
	}
	taskDef2 := task.Definition{
		Target:     "provider2",
		Definition: definition,
		ID:         uuid.New(),
		Type:       task.Retrieval,
	}
	resolver := new(module.MockDealStatesResolver)
	dispatcher := NewDispatcher(time.Second, resolver)
	inputs, err := dispatcher.GetTasks([]task.Definition{taskDef1, taskDef2})
	assert.NoError(err)
	assert.Equal(2, len(inputs))
	inputs, err = dispatcher.GetTasks([]task.Definition{taskDef1, taskDef2})
	assert.NoError(err)
	assert.Equal(0, len(inputs))
	time.Sleep(time.Second)
	inputs, err = dispatcher.GetTasks([]task.Definition{taskDef1, taskDef2})
	assert.NoError(err)
	assert.Equal(2, len(inputs))
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
	dispatcher := NewDispatcher(time.Second, new(module.MockDealStatesResolver))
	err = dispatcher.Validate(taskDef)
	assert.ErrorContains(err, "currently only GraphSync protocol is supported")
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
	auditor, err := NewAuditor(api, graphsync, 10*time.Second, 1, module.LocationFilterConfig{})
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
	assert.Equal([]string{"data cid is required for GraphSync protocol"}, out.AuditorErrors)
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
	auditor, err := NewAuditor(api, graphsync, 10*time.Second, 1, module.LocationFilterConfig{})
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
	assert.Equal(0, len(out.AuditorErrors))
	assert.Equal(QueryResponseError, out.Results[0].Status)
	assert.Contains(out.Results[0].ErrorMessage, "key not found")
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
		api, graphsync, 10*time.Second, 1, module.LocationFilterConfig{
			Continent: []string{"AAA"},
		},
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

func TestRetrieval_SuccessRetrieval(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	api, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
	defer closer()
	mockRetriever := new(MockGraphSyncRetriever)
	mockRetrieverBuilder := MockGraphSyncRetrieverBuilder{Retriever: mockRetriever}
	auditor, err := NewAuditor(api, &mockRetrieverBuilder, 10*time.Second, 1, module.LocationFilterConfig{})
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
	assert.Equal(0, len(out.AuditorErrors))
	assert.Equal(Success, out.Results[0].Status)
	assert.EqualValues(100, out.TotalBytesDownloaded)
	assert.EqualValues(10, out.MaxAverageSpeedPerSec)
	assert.Equal(2*time.Second, out.MinTimeToFirstByte)
}
