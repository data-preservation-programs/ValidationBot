package module

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/bcicen/jstream"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Deal struct {
	Proposal DealProposal
	State    DealState
}

type Cid struct {
	Root string `json:"/" mapstructure:"/"`
}

type DealProposal struct {
	PieceCID     Cid
	PieceSize    uint64
	VerifiedDeal bool
	Client       string
	Provider     string
	Label        string
	StartEpoch   int32
	EndEpoch     int32
}

type DealState struct {
	SectorStartEpoch int32
	LastUpdatedEpoch int32
	SlashEpoch       int32
}

type DealStatesResolver interface {
	DealsByProvider(provider string) ([]DealStateModel, error)
	DealsByProviderClients(provider string, clients []string) ([]DealStateModel, error)
}

type DealStateModel struct {
	DealID           string `gorm:"primaryKey"`
	PieceCid         string
	PieceSize        uint64
	VerifiedDeal     bool
	Provider         string `gorm:"index:idx_provider_client"`
	Client           string `gorm:"index:idx_provider_client"`
	Label            string
	StartEpoch       int32
	EndEpoch         int32
	SectorStartEpoch int32
	LastUpdatedEpoch int32
	SlashEpoch       int32
}

func (DealStateModel) TableName() string {
	return "deal_states"
}

type ClientAddressModel struct {
	Address string `gorm:"primaryKey"`
	ID      string
}

type GlifDealStatesResolver struct {
	url             string
	lotusAPI        api.Gateway
	db              *gorm.DB
	batchSize       int
	refreshInterval time.Duration
}

type ActorID = string

func (s *GlifDealStatesResolver) getAddressID(clientAddress string) (ActorID, error) {
	switch {
	case strings.HasPrefix(clientAddress, "f0"):
		return clientAddress, nil
	default:
		model := ClientAddressModel{}

		response := s.db.Model(&ClientAddressModel{}).Where("address = ?", clientAddress).First(&model)
		if response.Error == nil {
			return model.ID, nil
		}

		if !errors.Is(response.Error, gorm.ErrRecordNotFound) {
			return "", errors.Wrap(response.Error, "failed to query client address")
		}

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

		model = ClientAddressModel{
			Address: clientAddress,
			ID:      addressID.String(),
		}

		response = s.db.Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "address"}},
				DoNothing: true,
			},
		).Model(&ClientAddressModel{}).Create(&model)
		if response.Error != nil {
			return "", errors.Wrap(response.Error, "failed to create client address")
		}

		return model.ID, nil
	}
}

func currentEpoch() int32 {
	timestamp := time.Now().Unix()
	//nolint:gomnd
	return int32((timestamp - 1598306400) / 30)
}

func (s *GlifDealStatesResolver) DealsByProvider(provider string) ([]DealStateModel, error) {
	deals := make([]DealStateModel, 0)

	response := s.db.Model(&DealStateModel{}).
		Where(
			"provider = ? and end_epoch > ? and slash_epoch < 0 and sector_start_epoch > 0",
			provider, currentEpoch(),
		).
		Find(&deals)
	if response.Error != nil {
		return nil, errors.Wrap(response.Error, "failed to query deals by provider")
	}

	return deals, nil
}

func (s *GlifDealStatesResolver) DealsByProviderClients(provider string, clients []string) ([]DealStateModel, error) {
	deals := make([]DealStateModel, 0)
	ids := make([]ActorID, 0)

	for _, client := range clients {
		id, err := s.getAddressID(client)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get client address ID")
		}

		ids = append(ids, id)
	}

	response := s.db.Model(&DealStateModel{}).
		Where(
			"provider = ? AND client IN ? and end_epoch > ? and slash_epoch < 0 and sector_start_epoch > 0",
			provider, ids, currentEpoch(),
		).
		Find(&deals)
	if response.Error != nil {
		return nil, errors.Wrap(response.Error, "failed to query deals by provider")
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
	count := 0
	batch := make([]DealStateModel, 0)
	save := func(batch []DealStateModel) error {
		response := s.db.Clauses(
			clause.OnConflict{
				Columns: []clause.Column{{Name: "deal_id"}},
				DoUpdates: clause.AssignmentColumns(
					[]string{
						"sector_start_epoch", "last_updated_epoch", "slash_epoch",
					},
				),
			},
		).Create(&batch)

		if response.Error != nil {
			return errors.Wrap(response.Error, "failed to save deal state")
		}

		return nil
	}

	for stream := range decoder.Stream() {
		count++

		keyValuePair, ok := stream.Value.(jstream.KV)
		if !ok {
			return errors.New("unexpected stream value")
		}

		var deal Deal

		err = mapstructure.Decode(keyValuePair.Value, &deal)
		if err != nil {
			return errors.Wrap(err, "failed to decode deal")
		}

		model := DealStateModel{
			DealID:           keyValuePair.Key,
			PieceCid:         deal.Proposal.PieceCID.Root,
			PieceSize:        deal.Proposal.PieceSize,
			VerifiedDeal:     deal.Proposal.VerifiedDeal,
			Provider:         deal.Proposal.Provider,
			Client:           deal.Proposal.Client,
			Label:            deal.Proposal.Label,
			StartEpoch:       deal.Proposal.StartEpoch,
			EndEpoch:         deal.Proposal.EndEpoch,
			SectorStartEpoch: deal.State.SectorStartEpoch,
			LastUpdatedEpoch: deal.State.LastUpdatedEpoch,
			SlashEpoch:       deal.State.SlashEpoch,
		}

		batch = append(batch, model)
		if len(batch) >= s.batchSize {
			err = save(batch)
			if err != nil {
				return errors.Wrap(err, "failed to save batch")
			}

			batch = make([]DealStateModel, 0)
		}
	}

	if len(batch) > 0 {
		err = save(batch)
		if err != nil {
			return errors.Wrap(err, "failed to save batch")
		}
	}

	log.Debug().Dur("timeSpent", time.Since(now)).
		Int("count", count).Msg("refreshed deal states")

	if count == 0 {
		return errors.New("no deals downloaded")
	}

	return nil
}

func NewGlifDealStatesResolver(
	db *gorm.DB,
	lotusAPI api.Gateway,
	url string,
	refreshInterval time.Duration,
	batchSize int,
) (
	*GlifDealStatesResolver,
	error,
) {
	dealStates := GlifDealStatesResolver{
		url:             url,
		lotusAPI:        lotusAPI,
		db:              db,
		batchSize:       batchSize,
		refreshInterval: refreshInterval,
	}

	err := db.AutoMigrate(&DealStateModel{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to migrate deal state model")
	}

	err = db.AutoMigrate(&ClientAddressModel{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to migrate client address model")
	}

	return &dealStates, nil
}

func (s *GlifDealStatesResolver) Start(ctx context.Context) {
	go func() {
		log := log.With().Str("module", "dealstates").Caller().Logger()

		for {
			err := s.refresh(ctx)
			if err != nil {
				log.Error().Err(err).Msg("failed to refresh deal states")
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(s.refreshInterval):
			}
		}
	}()
}
