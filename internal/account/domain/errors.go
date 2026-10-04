package domain

import (
	"errors"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
)

var (
	ErrInvalidCurrency    = money.ErrInvalidCurrency
	ErrNotFound           = errors.New("account not found")
	ErrInvalidAccountType = errors.New("invalid account type")
	ErrInvalidBalance     = errors.New("invalid account balance")
)
