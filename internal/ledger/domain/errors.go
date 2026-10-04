package domain

import "errors"

var (
	ErrInvalidPosting     = errors.New("invalid ledger posting")
	ErrInvalidTransaction = errors.New("invalid ledger transaction")
	ErrUnbalanced         = errors.New("ledger transaction is unbalanced")
	ErrInsufficientFunds  = errors.New("insufficient posted funds")
)
