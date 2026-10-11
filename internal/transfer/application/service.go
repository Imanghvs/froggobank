package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	account "github.com/Imanghvs/froggobank/internal/account/domain"
	money "github.com/Imanghvs/froggobank/internal/money/domain"
	"github.com/Imanghvs/froggobank/internal/transfer/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type Service struct {
	repository    Repository
	accountReader AccountReader
}

func New(repository Repository, accountReader AccountReader) *Service {
	return &Service{
		repository:    repository,
		accountReader: accountReader,
	}
}

func (s *Service) Transfer(
	ctx context.Context,
	callerID uuid.UUID,
	source uuid.UUID,
	destination uuid.UUID,
	amountMinorCode string,
) (domain.Transfer, error) {
	if callerID == uuid.Nil {
		return domain.Transfer{}, user.ErrUnauthenticated
	}
	if source == uuid.Nil || destination == uuid.Nil {
		return domain.Transfer{}, domain.ErrInvalidAccountID
	}
	if source == destination {
		return domain.Transfer{}, domain.ErrSameAccount
	}
	amountMinor, err := strconv.ParseInt(amountMinorCode, 10, 64)
	if err != nil || amountMinor <= 0 {
		return domain.Transfer{}, domain.ErrInvalidAmount
	}

	if strconv.FormatInt(amountMinor, 10) != amountMinorCode {
		return domain.Transfer{}, domain.ErrInvalidAmount
	}
	sourceAccount, err := s.accountReader.GetAccountByID(ctx, callerID, source)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return domain.Transfer{}, domain.ErrSourceNotFound
		}
		return domain.Transfer{}, fmt.Errorf("read source account: %w", err)
	}
	destinationAccount, err := s.accountReader.GetAccountByID(ctx, callerID, destination)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return domain.Transfer{}, domain.ErrDestinationNotFound
		}
		return domain.Transfer{}, fmt.Errorf("read destination account: %w", err)
	}
	if sourceAccount.Currency != destinationAccount.Currency {
		return domain.Transfer{}, domain.ErrCurrencyMismatch
	}
	amount, err := money.New(amountMinor, sourceAccount.Currency)
	if err != nil {
		return domain.Transfer{}, err
	}
	tr, err := domain.New(source, destination, amount)
	if err != nil {
		return domain.Transfer{}, err
	}
	if err := s.repository.Create(ctx, tr); err != nil {
		return domain.Transfer{}, err
	}

	return tr, nil
}
