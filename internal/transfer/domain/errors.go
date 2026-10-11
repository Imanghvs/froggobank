package domain

import (
	"errors"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
)

var (
	ErrInvalidCurrency     = money.ErrInvalidCurrency
	ErrInvalidAmount       = errors.New("invalid amount")
	ErrInvalidAccountID    = errors.New("invalid source or destination account ID")
	ErrSameAccount         = errors.New("source and destination accounts must be different")
	ErrSourceNotFound      = errors.New("source account not found")
	ErrDestinationNotFound = errors.New("destination account not found")
	ErrCurrencyMismatch    = errors.New("account currencies must match the transfer currency")
)
