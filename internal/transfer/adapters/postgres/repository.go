package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Imanghvs/froggobank/internal/transfer/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) Create(
	ctx context.Context,
	tr domain.Transfer,
) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO transfers (id, source, destination, currency, amount_minor, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		tr.ID,
		tr.Source,
		tr.Destination,
		tr.Currency,
		tr.AmountMinor.MinorUnits(),
		tr.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create transfer: %w", err)
	}
	return nil
}
