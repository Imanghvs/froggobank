package domain

import "errors"

var (
	ErrInvalidCurrency = errors.New("invalid currency")
	ErrNotFound        = errors.New("account not found")
)
