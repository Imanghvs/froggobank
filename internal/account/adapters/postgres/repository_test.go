package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	testdb "github.com/Imanghvs/froggobank/internal/testutil/postgres"
)

func setupTestRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(t.Context()); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	repo := New(pool)

	return repo, pool
}

func cleanupAccount(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := pool.Exec(
			ctx,
			`DELETE FROM accounts WHERE id = $1`,
			id,
		); err != nil {
			t.Errorf("clean up account: %v", err)
		}
	})
}

func TestRepositoryCreate(t *testing.T) {
	repo, pool := setupTestRepository(t)

	acc := domain.New(domain.CurrencyEUR)

	err := repo.Create(t.Context(), acc)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	cleanupAccount(t, pool, acc.ID)
}

func TestRepositoryGetByIDReturnsNotFound(t *testing.T) {
	repo, _ := setupTestRepository(t)

	_, err := repo.GetByID(t.Context(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Error("PostgreSQL-specific not-found error must not leave the adapter")
	}
}

func TestRepositoryGetByID(t *testing.T) {
	repo, pool := setupTestRepository(t)

	acc := domain.New(domain.CurrencyEUR)

	err := repo.Create(t.Context(), acc)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	cleanupAccount(t, pool, acc.ID)

	retrievedAcc, err := repo.GetByID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("get account by id: %v", err)
	}
	if retrievedAcc.ID != acc.ID {
		t.Errorf("expected account id %q, received %q", acc.ID, retrievedAcc.ID)
	}
	if retrievedAcc.Currency != acc.Currency {
		t.Errorf("expected currency %q, received %q", acc.Currency, retrievedAcc.Currency)
	}
	if !retrievedAcc.CreatedAt.Equal(acc.CreatedAt) {
		t.Errorf("expected account created_at %q, received %q", acc.CreatedAt, retrievedAcc.CreatedAt)
	}

}

func TestRepositoryGetUserAccounts(t *testing.T) {
	pool := testdb.New(t)
	repo := New(pool)
	ownerID, otherOwnerID, emptyOwnerID := uuid.New(), uuid.New(), uuid.New()
	createdAt := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	for _, id := range []uuid.UUID{ownerID, otherOwnerID, emptyOwnerID} {
		if _, err := pool.Exec(t.Context(),
			`INSERT INTO users (id, issuer, subject, created_at) VALUES ($1, $2, $3, $4)`,
			id, "https://repository-test.example", id.String(), createdAt,
		); err != nil {
			t.Fatalf("create test user: %v", err)
		}
	}

	createAccount := func(id string, owner uuid.UUID, currency domain.Currency, timestamp time.Time) domain.Account {
		t.Helper()
		var acc domain.Account
		if owner == uuid.Nil {
			acc = domain.New(currency)
		} else {
			var err error
			acc, err = domain.NewCustomer(currency, owner)
			if err != nil {
				t.Fatalf("construct customer account: %v", err)
			}
		}
		acc.ID = uuid.MustParse(id)
		acc.CreatedAt = timestamp
		if err := repo.Create(t.Context(), acc); err != nil {
			t.Fatalf("create test account: %v", err)
		}
		return acc
	}

	// Insert in a different order from the expected created_at DESC, id DESC order.
	oldest := createAccount("00000000-0000-0000-0000-000000000001", ownerID, domain.CurrencyEUR, createdAt)
	newerLowID := createAccount("00000000-0000-0000-0000-000000000003", ownerID, domain.CurrencyGBP, createdAt.Add(2*time.Hour))
	middle := createAccount("00000000-0000-0000-0000-000000000002", ownerID, domain.CurrencyUSD, createdAt.Add(time.Hour))
	newerHighID := createAccount("00000000-0000-0000-0000-000000000004", ownerID, domain.CurrencyEUR, createdAt.Add(2*time.Hour))
	other := createAccount("00000000-0000-0000-0000-000000000005", otherOwnerID, domain.CurrencyUSD, createdAt.Add(3*time.Hour))
	createAccount("00000000-0000-0000-0000-000000000006", uuid.Nil, domain.CurrencyGBP, createdAt.Add(4*time.Hour))
	otherEUR := createAccount("00000000-0000-0000-0000-000000000007", otherOwnerID, domain.CurrencyEUR, createdAt.Add(5*time.Hour))
	ordered := []domain.Account{newerHighID, newerLowID, middle, oldest}
	eur, usd, gbp := domain.CurrencyEUR, domain.CurrencyUSD, domain.CurrencyGBP

	for _, tc := range []struct {
		name     string
		owner    uuid.UUID
		limit    int
		offset   int
		currency *domain.Currency
		want     []domain.Account
	}{
		{"all owned accounts", ownerID, 20, 0, nil, ordered},
		{"first page", ownerID, 2, 0, nil, ordered[:2]},
		{"offset within results", ownerID, 2, 1, nil, ordered[1:3]},
		{"partial final page", ownerID, 2, 3, nil, ordered[3:]},
		{"offset at end", ownerID, 2, 4, nil, nil},
		{"offset beyond end", ownerID, 2, 10, nil, nil},
		{"another owner's accounts", otherOwnerID, 20, 0, nil, []domain.Account{otherEUR, other}},
		{"owner without accounts", emptyOwnerID, 20, 0, nil, nil},
		{"unknown owner", uuid.New(), 20, 0, nil, nil},
		{"nil owner excludes internal accounts", uuid.Nil, 20, 0, nil, nil},
		{"EUR filter preserves order and ownership", ownerID, 20, 0, &eur, []domain.Account{newerHighID, oldest}},
		{"USD filter", ownerID, 20, 0, &usd, []domain.Account{middle}},
		{"GBP filter excludes internal accounts", ownerID, 20, 0, &gbp, []domain.Account{newerLowID}},
		{"currency filter before limit", ownerID, 1, 0, &usd, []domain.Account{middle}},
		{"currency filter before offset", ownerID, 1, 1, &eur, []domain.Account{oldest}},
		{"filtered offset at end", ownerID, 2, 2, &eur, nil},
		{"filtered offset beyond end", ownerID, 2, 10, &eur, nil},
		{"no matching currency", otherOwnerID, 20, 0, &gbp, nil},
		{"filtered owner without accounts", emptyOwnerID, 20, 0, &eur, nil},
		{"filtered nil owner excludes internal accounts", uuid.Nil, 20, 0, &gbp, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.GetUserAccounts(t.Context(), tc.owner, application.AccountFilter{
				Limit: tc.limit, Offset: tc.offset, Currency: tc.currency,
			})
			if err != nil {
				t.Fatalf("get user accounts: %v", err)
			}
			if got.Limit != tc.limit || got.Offset != tc.offset {
				t.Errorf("pagination metadata: got limit=%d offset=%d, want limit=%d offset=%d", got.Limit, got.Offset, tc.limit, tc.offset)
			}
			if got.Accounts == nil {
				t.Error("expected a non-nil accounts slice")
			}
			if len(got.Accounts) != len(tc.want) {
				t.Fatalf("got %d accounts, want %d", len(got.Accounts), len(tc.want))
			}
			for i, want := range tc.want {
				acc := got.Accounts[i]
				if acc.ID != want.ID || acc.OwnerID != want.OwnerID || acc.Currency != want.Currency ||
					acc.Type != want.Type || acc.EnforceNonnegativeBalance != want.EnforceNonnegativeBalance ||
					!acc.CreatedAt.Equal(want.CreatedAt) {
					t.Errorf("account at index %d: got %+v, want %+v", i, acc, want)
				}
			}
		})
	}

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := repo.GetUserAccounts(ctx, ownerID, application.AccountFilter{Limit: 20}); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}
