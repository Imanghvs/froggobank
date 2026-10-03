package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Currency string

const (
	CurrencyEUR Currency = "EUR"
	CurrencyUSD Currency = "USD"
	CurrencyGBP Currency = "GBP"
)

func (c Currency) IsSupported() bool {
	switch c {
	case CurrencyEUR, CurrencyUSD, CurrencyGBP:
		return true
	default:
		return false
	}
}

type Account struct {
	ID        uuid.UUID
	Currency  Currency
	CreatedAt time.Time
}

func ParseCurrency(value string) (Currency, error) {
	currency := Currency(value)

	if !currency.IsSupported() {
		return "", fmt.Errorf("%w: %q is not a supported currency", ErrInvalidCurrency, value)
	}

	return currency, nil
}

func New(currency Currency) Account {
	return Account{
		ID:        uuid.New(),
		Currency:  currency,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}
