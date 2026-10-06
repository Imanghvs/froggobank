package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/account/domain"
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
	var ownerID any
	if acc.OwnerID != uuid.Nil {
		ownerID = acc.OwnerID
	}
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO accounts (id, currency, created_at, account_type, enforce_nonnegative_balance, owner_id)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		acc.ID,
		acc.Currency,
		acc.CreatedAt,
		string(acc.Type),
		acc.EnforceNonnegativeBalance,
		ownerID,
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
		`SELECT id, currency, created_at, account_type, enforce_nonnegative_balance,
		COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid) FROM accounts
		WHERE id = $1`,
		id,
	).Scan(
		&acc.ID,
		&acc.Currency,
		&acc.CreatedAt,
		&acc.Type,
		&acc.EnforceNonnegativeBalance,
		&acc.OwnerID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrNotFound
		}
		return domain.Account{}, fmt.Errorf("get account by id: %w", err)
	}

	return acc, nil
}

func (r *Repository) GetBalance(ctx context.Context, id uuid.UUID) (domain.Balance, error) {
	var acc domain.Account
	var debitsText, creditsText string
	err := r.pool.QueryRow(
		ctx,
		`SELECT a.id, a.currency, a.account_type,
		b.debits_minor::text, b.credits_minor::text
		FROM accounts a LEFT JOIN account_balances b ON b.account_id = a.id
		WHERE a.id = $1`,
		id,
	).Scan(&acc.ID, &acc.Currency, &acc.Type, &debitsText, &creditsText)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Balance{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Balance{}, fmt.Errorf("get account balance: %w", err)
	}
	debits, debitOK := new(big.Int).SetString(debitsText, 10)
	credits, creditOK := new(big.Int).SetString(creditsText, 10)
	if !debitOK || !creditOK {
		return domain.Balance{}, domain.ErrInvalidBalance
	}
	return domain.NewBalance(acc, debits, credits)
}

func (r *Repository) GetUserAccounts(
	ctx context.Context,
	userID uuid.UUID,
	filter application.AccountFilter,
) (domain.PaginatedAccountsResponse, error) {
	accounts := []domain.Account{}
	query := `
		SELECT id, currency, created_at, account_type, enforce_nonnegative_balance, owner_id
		FROM accounts
		WHERE owner_id = $1`
	args := []any{userID, filter.Limit, filter.Offset}

	if filter.Currency != nil {
		query += ` AND currency = $4`
		args = append(args, *filter.Currency)
	}

	query += `
		ORDER BY created_at DESC, id DESC
		LIMIT $2
		OFFSET $3`

	rows, err := r.pool.Query(
		ctx,
		query,
		args...,
	)
	if err != nil {
		return domain.PaginatedAccountsResponse{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var acc domain.Account
		err := rows.Scan(
			&acc.ID,
			&acc.Currency,
			&acc.CreatedAt,
			&acc.Type,
			&acc.EnforceNonnegativeBalance,
			&acc.OwnerID,
		)
		if err != nil {
			return domain.PaginatedAccountsResponse{}, err
		}
		accounts = append(accounts, acc)
	}
	if err := rows.Err(); err != nil {
		return domain.PaginatedAccountsResponse{}, err
	}
	return domain.PaginatedAccountsResponse{
		Accounts: accounts,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	}, nil
}
