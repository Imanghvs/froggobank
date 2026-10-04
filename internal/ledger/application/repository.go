package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/ledger/domain"
)

// Repository appends a complete transaction and its postings and updates the
// affected account balance projections atomically.
// Implementations must never expose a partially persisted transaction.
type Repository interface {
	Post(ctx context.Context, transaction domain.Transaction) error
}
