package postgres

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	accountpostgres "github.com/Imanghvs/froggobank/internal/account/adapters/postgres"
	account "github.com/Imanghvs/froggobank/internal/account/domain"
	money "github.com/Imanghvs/froggobank/internal/money/domain"
	testdb "github.com/Imanghvs/froggobank/internal/testutil/postgres"
	"github.com/Imanghvs/froggobank/internal/transfer/domain"
)

func createTestAccount(t *testing.T, pool *pgxpool.Pool, currency money.Currency) uuid.UUID {
	t.Helper()
	acc := account.New(currency)
	if err := accountpostgres.New(pool).Create(t.Context(), acc); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acc.ID
}

func TestRepositoryCreate(t *testing.T) {
	pool := testdb.New(t)
	repo := New(pool)
	for _, tc := range []struct {
		name     string
		currency money.Currency
		amount   int64
	}{
		{"EUR", money.CurrencyEUR, 2500},
		{"USD", money.CurrencyUSD, 1},
		{"GBP", money.CurrencyGBP, 100},
		{"maximum amount", money.CurrencyEUR, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := createTestAccount(t, pool, tc.currency)
			destination := createTestAccount(t, pool, tc.currency)
			amount, err := money.New(tc.amount, tc.currency)
			if err != nil {
				t.Fatal(err)
			}
			tr, err := domain.New(source, destination, amount)
			if err != nil {
				t.Fatal(err)
			}
			tr.CreatedAt = time.Date(2026, time.January, 1, 12, 0, 0, 123456000, time.UTC)
			if err := repo.Create(t.Context(), tr); err != nil {
				t.Fatalf("create transfer: %v", err)
			}

			var id, gotSource, gotDestination uuid.UUID
			var currency string
			var amountMinor int64
			var createdAt time.Time
			err = pool.QueryRow(t.Context(),
				`SELECT id, source, destination, currency::text, amount_minor, created_at
				FROM transfers WHERE id = $1`, tr.ID,
			).Scan(&id, &gotSource, &gotDestination, &currency, &amountMinor, &createdAt)
			if err != nil {
				t.Fatalf("read transfer: %v", err)
			}
			if id != tr.ID || gotSource != source || gotDestination != destination ||
				currency != string(tc.currency) || amountMinor != tc.amount || !createdAt.Equal(tr.CreatedAt) {
				t.Fatalf("stored transfer differs: id=%s source=%s destination=%s currency=%s amount=%d created_at=%s",
					id, gotSource, gotDestination, currency, amountMinor, createdAt)
			}
		})
	}
}

func TestRepositoryCreateRejectsInvalidTransfers(t *testing.T) {
	pool := testdb.New(t)
	repo := New(pool)
	source := createTestAccount(t, pool, money.CurrencyEUR)
	destination := createTestAccount(t, pool, money.CurrencyEUR)
	usdAccount := createTestAccount(t, pool, money.CurrencyUSD)
	amount, err := money.New(2500, money.CurrencyEUR)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := domain.New(source, destination, amount)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(t.Context(), valid); err != nil {
		t.Fatalf("create initial transfer: %v", err)
	}
	for _, tc := range []struct {
		name       string
		modify     func(*domain.Transfer)
		code       string
		constraint string
	}{
		{"duplicate ID", func(tr *domain.Transfer) { tr.ID = valid.ID }, "23505", "transfers_pkey"},
		{"empty ID", func(tr *domain.Transfer) { tr.ID = uuid.Nil }, "23514", "transfers_id_check"},
		{"empty source", func(tr *domain.Transfer) { tr.Source = uuid.Nil }, "23514", "transfers_source_check"},
		{"empty destination", func(tr *domain.Transfer) { tr.Destination = uuid.Nil }, "23514", "transfers_destination_check"},
		{"identical accounts", func(tr *domain.Transfer) { tr.Destination = tr.Source }, "23514", "transfers_distinct_accounts"},
		{"missing source", func(tr *domain.Transfer) { tr.Source = uuid.New() }, "23503", "transfers_source_currency_fk"},
		{"missing destination", func(tr *domain.Transfer) { tr.Destination = uuid.New() }, "23503", "transfers_destination_currency_fk"},
		{"source currency mismatch", func(tr *domain.Transfer) { tr.Source = usdAccount }, "23503", "transfers_source_currency_fk"},
		{"destination currency mismatch", func(tr *domain.Transfer) { tr.Destination = usdAccount }, "23503", "transfers_destination_currency_fk"},
		{"zero amount", func(tr *domain.Transfer) {
			tr.AmountMinor, _ = money.New(0, money.CurrencyEUR)
		}, "23514", "transfers_amount_minor_check"},
		{"negative amount", func(tr *domain.Transfer) {
			tr.AmountMinor, _ = money.New(-1, money.CurrencyEUR)
		}, "23514", "transfers_amount_minor_check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := domain.New(source, destination, amount)
			if err != nil {
				t.Fatal(err)
			}
			tc.modify(&tr)
			// Bypass constructor guarantees to verify the database constraints.
			err = repo.Create(t.Context(), tr)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code || pgErr.ConstraintName != tc.constraint {
				t.Fatalf("error = %v, want PostgreSQL %s for %s", err, tc.code, tc.constraint)
			}
		})
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	tr, err := domain.New(source, destination, amount)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, tr); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	var count int
	if err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM transfers`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("failed inserts left rows: count = %d, want 1", count)
	}
}
