package domain

import (
	"time"

	"github.com/google/uuid"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

// Currency shares the supported-currency rules with the money domain.
type Currency = money.Currency

const (
	CurrencyEUR = money.CurrencyEUR
	CurrencyUSD = money.CurrencyUSD
	CurrencyGBP = money.CurrencyGBP
)

type Account struct {
	ID       uuid.UUID
	Currency Currency
	Type     AccountType
	// Nil marks an internal ledger account that customers cannot access.
	OwnerID uuid.UUID
	// Generic ledger accounts are unrestricted unless this policy is enabled.
	EnforceNonnegativeBalance bool
	CreatedAt                 time.Time
}

func NewCustomer(currency Currency, ownerID uuid.UUID) (Account, error) {
	if ownerID == uuid.Nil {
		return Account{}, user.ErrUnauthenticated
	}
	acc, err := NewWithType(currency, Liability)
	if err != nil {
		return Account{}, err
	}
	acc.OwnerID = ownerID
	acc.EnforceNonnegativeBalance = true
	return acc, nil
}

func ParseCurrency(value string) (Currency, error) {
	return money.ParseCurrency(value)
}

func New(currency Currency) Account {
	return Account{
		ID:        uuid.New(),
		Currency:  currency,
		Type:      Liability,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}

func NewWithType(currency Currency, accountType AccountType) (Account, error) {
	if _, err := currency.Scale(); err != nil {
		return Account{}, err
	}
	if _, err := accountType.NormalSide(); err != nil {
		return Account{}, err
	}
	acc := New(currency)
	acc.Type = accountType
	return acc, nil
}
