package module

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bcicen/jstream"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
)

type Deal struct {
	Proposal DealProposal
	State    DealState
}

type Cid struct {
	Root string `json:"/" mapstructure:"/"`
}

type DealProposal struct {
	PieceCID Cid
	Client   string
	Provider string
	Label    string
}

type DealState struct {
	SectorStartEpoch int64
	SlashEpoch       int64
}

type SimplifiedDeal struct {
	PieceCID string
	Label    string
}

type DealStatesResolver interface {
	Ready() bool
	DealsByProvider(provider string) []SimplifiedDeal
	DealsByProviderClient(provider string, client string) ([]SimplifiedDeal, error)
	DealsByProviderClients(provider string, clients []string) ([]SimplifiedDeal, error)
}

type MockDealStatesResolver struct {
	mock.Mock
}

func (s *MockDealStatesResolver) Ready() bool {
	return true
}

//nolint:all
func (m *MockDealStatesResolver) DealsByProvider(provider string) []SimplifiedDeal {
	args := m.Called(provider)
	return args.Get(0).([]SimplifiedDeal)
}

//nolint:all
func (m *MockDealStatesResolver) DealsByProviderClient(provider string, client string) ([]SimplifiedDeal, error) {
	args := m.Called(provider, client)
	return args.Get(0).([]SimplifiedDeal), args.Error(1)
}

//nolint:all
func (m *MockDealStatesResolver) DealsByProviderClients(provider string, clients []string) ([]SimplifiedDeal, error) {
	args := m.Called(provider, clients)
	return args.Get(0).([]SimplifiedDeal), args.Error(1)
}

type GlifDealStatesResolver struct {
	url              string
	byProviderClient map[string]map[string][]SimplifiedDeal
	lotusAPI         api.Gateway
	idLookupCache    map[string]string
	cacheMutex       sync.RWMutex
	ready            bool
}

func (s *GlifDealStatesResolver) Ready() bool {
	return s.ready
}

func (s *GlifDealStatesResolver) getAddressID(clientAddress string) (string, error) {
	switch {
	case strings.HasPrefix(clientAddress, "f0"):
		return clientAddress, nil
	default:
		s.cacheMutex.RLock()
		if id, ok := s.idLookupCache[clientAddress]; ok {
			s.cacheMutex.RUnlock()
			return id, nil
		}
		s.cacheMutex.RUnlock()

		addr, err := address.NewFromString(clientAddress)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse clientAddress address")
		}

		log.Debug().Str("role", "lotus_api").
			Str("method", "StateLookupID").Str("address", clientAddress).Msg("calling lotus api")

		addressID, err := s.lotusAPI.StateLookupID(context.Background(), addr, types.EmptyTSK)
		if err != nil {
			return "", errors.Wrap(err, "failed to lookup clientAddress ID")
		}

		s.cacheMutex.Lock()
		defer s.cacheMutex.Unlock()
		s.idLookupCache[clientAddress] = addressID.String()
		return addressID.String(), nil
	}
}

func (s *GlifDealStatesResolver) DealsByProvider(provider string) []SimplifiedDeal {
	deals := make([]SimplifiedDeal, 0)
	for _, clientDeals := range s.byProviderClient[provider] {
		deals = append(deals, clientDeals...)
	}
	return deals
}

func (s *GlifDealStatesResolver) DealsByProviderClient(provider string, client string) ([]SimplifiedDeal, error) {
	client, err := s.getAddressID(client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get client ID: %s", client)
	}

	if _, ok := s.byProviderClient[provider]; !ok {
		return make([]SimplifiedDeal, 0), nil
	} else if _, ok := s.byProviderClient[provider][client]; !ok {
		return make([]SimplifiedDeal, 0), nil
	}

	return s.byProviderClient[provider][client], nil
}

func (s *GlifDealStatesResolver) DealsByProviderClients(provider string, clients []string) ([]SimplifiedDeal, error) {
	deals := make([]SimplifiedDeal, 0)

	for _, client := range clients {
		client, err := s.getAddressID(client)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get client ID: %s", client)
		}

		if _, ok := s.byProviderClient[provider]; !ok {
			continue
		} else if _, ok := s.byProviderClient[provider][client]; !ok {
			continue
		}

		deals = append(deals, s.byProviderClient[provider][client]...)
	}

	return deals, nil
}

func (s *GlifDealStatesResolver) refresh(ctx context.Context) error {
	log := log.With().Str("module", "dealstates").Caller().Logger()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	log.Debug().Str("url", s.url).Msg("refreshing deal states")

	now := time.Now()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to make request")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	decoder := jstream.NewDecoder(resp.Body, 1).EmitKV()
	active := 0
	inactive := 0
	states := make(map[string]map[string][]SimplifiedDeal)

	for stream := range decoder.Stream() {
		keyValuePair, ok := stream.Value.(jstream.KV)
		if !ok {
			return errors.New("unexpected stream value")
		}

		var deal Deal

		err = mapstructure.Decode(keyValuePair.Value, &deal)
		if err != nil {
			return errors.Wrap(err, "failed to decode deal")
		}

		if deal.State.SlashEpoch > 0 || deal.State.SectorStartEpoch < 0 {
			inactive++
			continue
		}

		if _, ok := states[deal.Proposal.Provider]; !ok {
			states[deal.Proposal.Provider] = make(map[string][]SimplifiedDeal)
		}

		if _, ok := states[deal.Proposal.Provider][deal.Proposal.Client]; !ok {
			states[deal.Proposal.Provider][deal.Proposal.Client] = make([]SimplifiedDeal, 0)
		}

		states[deal.Proposal.Provider][deal.Proposal.Client] =
			append(
				states[deal.Proposal.Provider][deal.Proposal.Client], SimplifiedDeal{
					PieceCID: deal.Proposal.PieceCID.Root,
					Label:    deal.Proposal.Label,
				},
			)
		active++
	}

	s.byProviderClient = states

	log.Debug().Int("inactive_deals", inactive).Dur("timeSpent", time.Since(now)).
		Int("active_deals", active).Msg("refreshed deal states")

	if active == 0 {
		return errors.New("no active deals")
	}

	s.ready = true
	return nil
}

func NewDealStatesResolver(ctx context.Context, lotusAPI api.Gateway, url string, refreshInterval time.Duration) (
	*GlifDealStatesResolver,
	error,
) {
	dealStates := GlifDealStatesResolver{
		url:              url,
		byProviderClient: nil,
		lotusAPI:         lotusAPI,
		idLookupCache:    make(map[string]string),
		cacheMutex:       sync.RWMutex{},
		ready:            false,
	}

	go func() {
		log := log.With().Str("module", "dealstates").Caller().Logger()

		for {
			err := dealStates.refresh(ctx)
			if err != nil {
				log.Error().Err(err).Msg("failed to refresh deal states")
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(refreshInterval):
			}
		}
	}()

	return &dealStates, nil
}
