package domain

import "errors"

var (
	ErrInvalidCurrency  = errors.New("invalid currency")
	ErrCurrencyMismatch = errors.New("money currencies differ")
	ErrOverflow         = errors.New("money arithmetic overflow")
)
