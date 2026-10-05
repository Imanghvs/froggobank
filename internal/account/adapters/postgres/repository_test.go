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

	"github.com/Imanghvs/froggobank/internal/account/domain"
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
