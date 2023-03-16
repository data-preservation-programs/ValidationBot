package traceroute

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"validation-bot/task"

	"validation-bot/module"

	"github.com/filecoin-project/lotus/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Dispatcher struct {
	module.SimpleDispatcher
	module.NoopValidator
}

func (Dispatcher) Type() task.Type {
	return task.Traceroute
}

type Auditor struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
	useSudo  bool
}

func (Auditor) Type() task.Type {
	return task.Traceroute
}

func NewAuditor(lotusAPI api.Gateway, useSudo bool) Auditor {
	return Auditor{
		log:      log2.With().Str("role", "traceroute_auditor").Caller().Logger(),
		lotusAPI: lotusAPI,
		useSudo:  useSudo,
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

		hops, err := q.Traceroute(ctx, host, port, q.useSudo)
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

func (q Auditor) Traceroute(ctx context.Context, ip string, port int, useSudo bool) ([]Hop, error) {
	// Create a ConsoleWriter output writer that writes to standard output
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}

	// Create a new logger with the ConsoleWriter output writer
	log := zerolog.New(consoleWriter).With().Str("role", "traceroute").Caller().Timestamp().Logger()

	log.Info().Msg("start traceroute")
	cmdStr := fmt.Sprintf("./traceroute -TF -m 64 -p %d %s | jc --traceroute", port, ip)
	if useSudo {
		cmdStr = fmt.Sprintf("sudo %s", cmdStr)
	}

	log.Info().Msgf("running command: %s", cmdStr)
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)

	log.Info().Msg("waiting for command to finish")
	outputStr, err := cmd.CombinedOutput()
	log.Info().Msgf("command finished: %s", string(outputStr))
	if err != nil {
		log.Error().Err(err).Msg("failed to run traceroute")
		return nil, errors.Wrap(err, "failed to run traceroute")
	}

	var output Output

	log.Info().Msg("unmarshaling traceroute output")
	err = json.Unmarshal(outputStr, &output)
	if err != nil {
		errStr := strings.ReplaceAll(string(outputStr), "\n", "")
		return nil, errors.Wrapf(err, "failed to unmarshal traceroute output: %s", errStr)
	}

	return output.Hops, nil
}

func (q Auditor) ShouldValidate(ctx context.Context, input module.ValidationInput) (bool, error) {
	return true, nil
}
