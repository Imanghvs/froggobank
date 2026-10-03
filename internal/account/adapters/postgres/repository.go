package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

var _ application.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) Create(ctx context.Context, acc domain.Account) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO accounts (id, currency, created_at)
		VALUES ($1, $2, $3)`,
		acc.ID,
		acc.Currency,
		acc.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create account: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	var acc domain.Account

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, currency, created_at FROM accounts
		WHERE id = $1`,
		id,
	).Scan(
		&acc.ID,
		&acc.Currency,
		&acc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrNotFound
		}
		return domain.Account{}, fmt.Errorf("get account by id: %w", err)
	}

	return acc, nil
}
