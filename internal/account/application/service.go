package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateAccount(ctx context.Context, callerID uuid.UUID, currencyCode string) (domain.Account, error) {
	if callerID == uuid.Nil {
		return domain.Account{}, user.ErrUnauthenticated
	}
	currency, err := domain.ParseCurrency(currencyCode)
	if err != nil {
		return domain.Account{}, err
	}

	acc, err := domain.NewCustomer(currency, callerID)
	if err != nil {
		return domain.Account{}, err
	}
	if err := s.repository.Create(ctx, acc); err != nil {
		return domain.Account{}, err
	}

	return acc, nil
}

func (s *Service) GetAccountBalance(ctx context.Context, callerID, id uuid.UUID) (domain.Balance, error) {
	// Ownership is checked before reading balances. Assigned owners and account
	// policy are immutable in PostgreSQL, so this check cannot race with reassignment.
	if _, err := s.GetAccountByID(ctx, callerID, id); err != nil {
		return domain.Balance{}, err
	}
	return s.repository.GetBalance(ctx, id)
}

func (s *Service) GetAccountByID(ctx context.Context, callerID, id uuid.UUID) (domain.Account, error) {
	if callerID == uuid.Nil {
		return domain.Account{}, user.ErrUnauthenticated
	}
	acc, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return domain.Account{}, err
	}
	if acc.OwnerID != callerID || acc.Type != domain.Liability || !acc.EnforceNonnegativeBalance {
		return domain.Account{}, domain.ErrNotFound
	}
	return acc, nil
}
