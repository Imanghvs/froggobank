package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateAccount(ctx context.Context, currencyCode, accountTypeCode string, enforceNonnegativeBalance bool) (domain.Account, error) {
	currency, err := domain.ParseCurrency(currencyCode)
	if err != nil {
		return domain.Account{}, err
	}

	if accountTypeCode == "" {
		accountTypeCode = string(domain.Liability)
	}
	accountType, err := domain.ParseAccountType(accountTypeCode)
	if err != nil {
		return domain.Account{}, err
	}
	acc, err := domain.NewWithType(currency, accountType)
	if err != nil {
		return domain.Account{}, err
	}
	acc.EnforceNonnegativeBalance = enforceNonnegativeBalance
	if err := s.repository.Create(ctx, acc); err != nil {
		return domain.Account{}, err
	}

	return acc, nil
}

func (s *Service) GetAccountBalance(ctx context.Context, id uuid.UUID) (domain.Balance, error) {
	return s.repository.GetBalance(ctx, id)
}

func (s *Service) GetAccountByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return s.repository.GetByID(ctx, id)
}
