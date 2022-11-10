package traceroute

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/filecoin-project/lotus/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Dispatcher struct {
	module.SimpleDispatcher
}

func (d Dispatcher) Validate(definition task.Definition) error {
	return nil
}

type Auditor struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
}

func NewAuditor(lotusAPI api.Gateway) Auditor {
	return Auditor{
		log:      log2.With().Str("role", "traceroute_auditor").Caller().Logger(),
		lotusAPI: lotusAPI,
	}
}

func (q Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	provider := input.Target
	log := q.log.With().Str("provider", provider).Logger()

	log.Info().Msg("start validating traceroute")

	result, err := q.ValidateProvider(ctx, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate provider")
	}

	jsonb, err := module.NewJSONB(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return &module.ValidationResult{
		ValidationInput: input,
		Result:          jsonb,
	}, nil
}

func (q Auditor) ValidateProvider(ctx context.Context, provider string) (*ResultContent, error) {
	log := q.log.With().Str("provider", provider).Logger()
	log.Debug().Msg("querying miner info")

	minerInfoResult, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	if minerInfoResult.ErrorCode != "" {
		return &ResultContent{
			Status:       Status(minerInfoResult.ErrorCode),
			ErrorMessage: minerInfoResult.ErrorMessage,
		}, nil
	}

	if len(minerInfoResult.MultiAddrs) == 0 {
		return &ResultContent{
			Status:       NoMultiAddress,
			ErrorMessage: "miner has no multi address",
		}, nil
	}

	traces := make(map[string]HopResult)

	for _, multiAddr := range minerInfoResult.MultiAddrs {
		host, _, port, err := module.ResolveHostAndIP(multiAddr)
		if err != nil {
			//nolint:nilerr
			return &ResultContent{
				Status:       InvalidMultiAddress,
				ErrorMessage: "miner multiaddress is not resolvable",
			}, nil
		}

		hops, err := q.Traceroute(ctx, host, port)
		if err != nil {
			return nil, errors.Wrap(err, "failed to traceroute")
		}

		traces[multiAddr.String()] = HopResult{
			Hops:              hops,
			LastHopOverheadMs: calculateLastHopOverhead(hops),
		}
	}

	return &ResultContent{
		Status: Success,
		Traces: traces,
	}, nil
}

func calculateLastHopOverhead(hops []Hop) float64 {
	if len(hops) <= 1 {
		return 0
	}

	lastIPs := make([]string, 0)
	for _, probe := range hops[len(hops)-1].Probes {
		lastIPs = append(lastIPs, probe.IP)
	}

	var firstRTT float64

	for _, hop := range hops {
		firstMet := false

		for _, probe := range hop.Probes {
			if slices.Contains(lastIPs, probe.IP) {
				if firstRTT == 0 || probe.RTT < firstRTT {
					firstRTT = probe.RTT
					firstMet = true
				}
			}
		}

		if firstMet {
			break
		}
	}

	var lastRTT float64
	for _, probe := range hops[len(hops)-1].Probes {
		if lastRTT == 0 || probe.RTT < lastRTT {
			lastRTT = probe.RTT
		}
	}

	return lastRTT - firstRTT
}

func (q Auditor) Traceroute(ctx context.Context, ip string, port int) ([]Hop, error) {
	cmdStr := fmt.Sprintf("sudo traceroute -TF -m 64 -p %d %s | jc --traceroute", port, ip)
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)

	outputStr, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run traceroute")
	}

	var output Output

	err = json.Unmarshal(outputStr, &output)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal traceroute output: %s", string(outputStr))
	}

	return output.Hops, nil
}
