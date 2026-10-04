package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Imanghvs/froggobank/internal/ledger/application"
	"github.com/Imanghvs/froggobank/internal/ledger/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

var _ application.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Post(ctx context.Context, transaction domain.Transaction) error {
	if err := transaction.Validate(); err != nil {
		return err
	}
	postings := transaction.Postings()
	postingIDs := make([]uuid.UUID, len(postings))
	accountIDs := make([]uuid.UUID, len(postings))
	sides := make([]string, len(postings))
	minorUnits := make([]int64, len(postings))
	currencies := make([]string, len(postings))
	for i, posting := range postings {
		postingIDs[i] = posting.ID()
		accountIDs[i] = posting.AccountID()
		sides[i] = string(posting.Side())
		minorUnits[i] = posting.Amount().MinorUnits()
		currencies[i] = string(posting.Amount().Currency())
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin ledger transaction: %w", err)
	}
	defer func() {
		// A canceled request must not prevent cleanup of an open DB transaction.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO ledger_transactions (id, created_at, posting_count)
		VALUES ($1, $2, $3)`,
		transaction.ID(),
		transaction.CreatedAt(),
		int64(len(postings)),
	)
	if err != nil {
		return fmt.Errorf("insert ledger transaction: %w", err)
	}
	// Equal-length typed arrays preserve each posting's fields while keeping
	// the statement and parameter count fixed for every transaction size.
	// The statement-level database trigger aggregates these postings and updates
	// balance projections under row locks in account-ID order, in this same tx.
	_, err = tx.Exec(
		ctx,
		`INSERT INTO ledger_postings
		(id, transaction_id, account_id, side, amount_minor, currency)
		SELECT p.id, $1, p.account_id, p.side, p.amount_minor, p.currency::currency_code
		FROM unnest($2::uuid[], $3::uuid[], $4::text[], $5::bigint[], $6::text[])
		AS p(id, account_id, side, amount_minor, currency)`,
		transaction.ID(), postingIDs, accountIDs, sides, minorUnits, currencies,
	)
	if err != nil {
		return persistenceError("insert ledger postings", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return persistenceError("commit ledger transaction", err)
	}
	return nil
}

func persistenceError(operation string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "account_balances_nonnegative" {
		return fmt.Errorf("%s: %w: %w", operation, domain.ErrInsufficientFunds, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
