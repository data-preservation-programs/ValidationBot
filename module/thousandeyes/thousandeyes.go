package thousandeyes

import (
	"context"
	"encoding/json"
	"strconv"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/go-resty/resty/v2"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Dispatcher struct {
	module.SimpleDispatcher
}

func (Dispatcher) TaskType() task.Type {
	return task.QueryAsk
}

type AuditorModule struct {
	log      zerolog.Logger
	lotusAPI v0api.Gateway
	client   *resty.Client
	agents   []AgentID
}

func (AuditorModule) TaskType() task.Type {
	return task.ThousandEyes
}

func getHostAndIP(addr multiaddr.Multiaddr) (string, int, error) {
	protocols := addr.Protocols()
	const expectedProtocolCount = 2
	if len(protocols) != expectedProtocolCount {
		return "", 0, errors.New("multiaddr does not contain two protocols")
	}
	if !slices.Contains([]int{
		multiaddr.P_IP4, multiaddr.P_IP6,
		multiaddr.P_DNS4, multiaddr.P_DNS6,
		multiaddr.P_DNS, multiaddr.P_DNSADDR,
	}, protocols[0].Code) {
		return "", 0, errors.New("multiaddr does not contain a valid ip or dns protocol")
	}
	if protocols[1].Code != multiaddr.P_TCP {
		return "", 0, errors.New("multiaddr does not contain a valid tcp protocol")
	}

	splitted := multiaddr.Split(addr)
	component0, ok := splitted[0].(*multiaddr.Component)
	if !ok {
		return "", 0, errors.New("failed to cast component")
	}
	host := component0.Value()
	component1, ok := splitted[1].(*multiaddr.Component)
	if !ok {
		return "", 0, errors.New("failed to cast component")
	}
	port, err := strconv.Atoi(component1.Value())
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to parse port")
	}
	return host, port, nil
}

func (a AuditorModule) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	provider := input.Target

	minerInfoResult, err := module.GetMinerInfo(ctx, a.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	validationResult := make(ResultMap)
	if minerInfoResult.ErrorCode != "" {
		return nil, errors.New("failed to get miner info")
	}
	type TestResult struct {
		metrics   []Metric
		err       error
		multiAddr multiaddr.Multiaddr
	}
	resultChannels := make([]chan TestResult, 0)
	results := make([]TestResult, 0)
	for _, addr := range minerInfoResult.MultiAddrs {
		addr := addr
		host, port, err := getHostAndIP(addr)
		if err != nil {
			a.log.Error().Err(err).Msg("failed to get host and port from multiaddr")
			continue
		}
		ch := make(chan TestResult)
		resultChannels = append(resultChannels, ch)
		go func() {
			testID, err := a.invokeNetworkTest(ctx, host, port)
			if err != nil {
				ch <- TestResult{
					metrics:   nil,
					err:       err,
					multiAddr: nil,
				}
			}
			metrics, err := a.retrieveTestResult(ctx, testID)
			if err != nil {
				ch <- TestResult{
					metrics:   nil,
					err:       err,
					multiAddr: nil,
				}
			}
			ch <- TestResult{
				metrics:   metrics,
				multiAddr: addr,
				err:       nil,
			}
		}()
	}
	for _, ch := range resultChannels {
		result := <-ch
		if result.err != nil {
			a.log.Error().Err(result.err).Msg("failed to retrieve test result")
		} else {
			results = append(results, result)
		}
	}
	for _, result := range results {
		validationResult[MultiAddrStr(result.multiAddr.String())] = ResultContent{
			Metrics:    result.metrics,
			MinLatency: 0,
			// TODO
			//nolint:exhaustruct
			ClosestAgent: Agent{},
		}
	}

	result, err := module.NewJSONB(validationResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create jsonb")
	}

	out := module.ValidationResult{
		Task:   input.Task,
		Result: result,
	}
	return &out, nil
}

func (a AuditorModule) invokeNetworkTest(ctx context.Context, server string, port int) (int, error) {
	response, err := a.client.R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("format", "json").
		SetBody(InvokeInstantTestRequest{
			Port:     port,
			Server:   server,
			Protocol: "tcp",
			Agents:   a.agents,
		}).Post("https://api.thousandeyes.com/v6/instant/agent-to-server")
	if err != nil {
		return 0, errors.Wrap(err, "failed to invoke network test")
	}
	var invokeResponse InvokeInstantTestResponse
	err = json.Unmarshal(response.Body(), &invokeResponse)
	if err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal invoke network test response")
	}

	if len(invokeResponse.Test) == 1 {
		return invokeResponse.Test[0].TestID, nil
	}

	return 0, errors.New("response from instant test does not contain exactly one test id")
}

func (a AuditorModule) retrieveTestResult(ctx context.Context, testID int) ([]Metric, error) {
	response, err := a.client.R().SetContext(ctx).
		SetQueryParam("format", "json").
		Get("https://api.thousandeyes.com/v6/net/metrics" + strconv.Itoa(testID))
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve test result")
	}
	var testResponse RetrieveTestResultResponse
	err = json.Unmarshal(response.Body(), &testResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal test result")
	}

	return testResponse.Net.Metrics, nil
}

func NewAuditorModuleWithAuthToken(lotusAPI v0api.Gateway, token string, agents []AgentID) AuditorModule {
	client := resty.New()
	client.SetAuthToken(token)
	return AuditorModule{
		log:      log.With().Str("module", "thousand_eyes_auditor").Logger(),
		lotusAPI: lotusAPI,
		client:   client,
		agents:   agents,
	}
}

func NewAuditorModuleWithBasicAuth(lotusAPI v0api.Gateway, username, password string, agents []AgentID) AuditorModule {
	client := resty.New()
	client.SetBasicAuth(username, password)
	return AuditorModule{
		log:      log.With().Str("module", "thousand_eyes_auditor").Logger(),
		lotusAPI: lotusAPI,
		client:   client,
		agents:   agents,
	}
}
