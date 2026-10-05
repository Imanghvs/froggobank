package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Imanghvs/froggobank/internal/user/application"
	"github.com/Imanghvs/froggobank/internal/user/domain"
)

type Repository struct{ pool *pgxpool.Pool }

var _ application.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) FindOrCreate(ctx context.Context, candidate domain.User) (domain.User, error) {
	if err := candidate.Identity.Validate(); err != nil {
		return domain.User{}, err
	}
	var user domain.User
	err := r.pool.QueryRow(ctx, `INSERT INTO users (id, issuer, subject, created_at)
		VALUES ($1, $2, $3, $4) ON CONFLICT (issuer, subject) DO NOTHING
		RETURNING id, created_at`, candidate.ID, candidate.Identity.Issuer(), candidate.Identity.Subject(), candidate.CreatedAt).Scan(&user.ID, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Use a new READ COMMITTED snapshot after the conflicting insert has
		// finished. A SELECT in the same statement might not see that commit.
		err = r.pool.QueryRow(ctx, `SELECT id, created_at FROM users WHERE issuer=$1 AND subject=$2`,
			candidate.Identity.Issuer(), candidate.Identity.Subject()).Scan(&user.ID, &user.CreatedAt)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("resolve local user: %w", err)
	}
	user.Identity = candidate.Identity
	return user, nil
}
