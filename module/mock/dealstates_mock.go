package mock

import (
	"validation-bot/module"

	"github.com/stretchr/testify/mock"
)

type MockDealStatesResolver struct {
	mock.Mock
}

//nolint:all
func (m *MockDealStatesResolver) DealsByProvider(provider string) ([]module.DealStateModel, error) {
	args := m.Called(provider)
	return args.Get(0).([]module.DealStateModel), args.Error(1)
}

//nolint:all
func (m *MockDealStatesResolver) DealsByProviderClients(provider string, clients []string) (
	[]module.DealStateModel,
	error,
) {
	args := m.Called(provider, clients)
	return args.Get(0).([]module.DealStateModel), args.Error(1)
}
