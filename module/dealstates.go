package module

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/go-resty/resty/v2"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
)

type Deal struct {
	Proposal DealProposal
	State    DealState
}

type DealProposal struct {
	PieceCID cid.Cid
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
	DealsByProvider(provider string) []SimplifiedDeal
	DealsByProviderClient(provider string, client string) ([]SimplifiedDeal, error)
	DealsByProviderClients(provider string, clients []string) ([]SimplifiedDeal, error)
}

type MockDealStatesResolver struct {
	mock.Mock
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
}

func (s *GlifDealStatesResolver) getAddressID(clientAddress string) (string, error) {
	switch {
	case strings.HasPrefix(clientAddress, "f0"):
		return clientAddress, nil
	default:
		s.cacheMutex.RLock()
		if id, ok := s.idLookupCache[clientAddress]; ok {
			return id, nil
		}
		s.cacheMutex.RUnlock()

		addr, err := address.NewFromString(clientAddress)
		if err != nil {
			return "", errors.Wrap(err, "failed to parse clientAddress address")
		}

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
	client := resty.New()

	log.Debug().Str("url", s.url).Msg("refreshing deal states")
	got, err := client.R().SetContext(ctx).Get(s.url)

	if got != nil {
		log.Debug().Str("url", s.url).Dur("timeSpent", got.Time()).Str(
			"status",
			got.Status(),
		).Msg("deal states response received")
	}

	if err != nil {
		return errors.Wrap(err, "failed to get deal states")
	}

	deals := make(map[string]Deal)

	err = json.Unmarshal(got.Body(), &deals)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal deal states")
	}

	active := 0
	inactive := 0
	states := make(map[string]map[string][]SimplifiedDeal)

	for _, deal := range deals {
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
					PieceCID: deal.Proposal.PieceCID.String(),
					Label:    deal.Proposal.Label,
				},
			)
		active++
	}

	s.byProviderClient = states

	log.Debug().Int("inactive_deals", inactive).Int("active_deals", active).Msg("refreshed deal states")
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
	}

	err := dealStates.refresh(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to refresh deal states")
	}

	go func() {
		log := log.With().Str("module", "dealstates").Caller().Logger()

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(refreshInterval):
				err := dealStates.refresh(ctx)
				if err != nil {
					log.Error().Err(err).Msg("failed to refresh deal states")
				}
			}
		}
	}()

	return &dealStates, nil
}
