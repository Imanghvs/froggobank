package domain

import (
	"time"

	"github.com/google/uuid"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
)

type Currency = money.Currency
type Transfer struct {
	ID          uuid.UUID
	Source      uuid.UUID
	Destination uuid.UUID
	AmountMinor money.Money
	Currency    Currency
	CreatedAt   time.Time
}

func ParseCurrency(value string) (Currency, error) {
	return money.ParseCurrency(value)
}

func New(
	source uuid.UUID,
	destination uuid.UUID,
	amountMinor money.Money,
) (Transfer, error) {
	if source == uuid.Nil || destination == uuid.Nil {
		return Transfer{}, ErrInvalidAccountID
	}
	if source == destination {
		return Transfer{}, ErrSameAccount
	}
	if err := amountMinor.Validate(); err != nil {
		return Transfer{}, err
	}
	if amountMinor.MinorUnits() <= 0 {
		return Transfer{}, ErrInvalidAmount
	}

	return Transfer{
		ID:          uuid.New(),
		Source:      source,
		Destination: destination,
		Currency:    amountMinor.Currency(),
		AmountMinor: amountMinor,
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}, nil
}
