package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Imanghvs/froggobank/internal/ledger/domain"
	money "github.com/Imanghvs/froggobank/internal/money/domain"
	"github.com/google/uuid"
)

type fakeRepository struct {
	postFn func(context.Context, domain.Transaction) error
}

func (f fakeRepository) Post(ctx context.Context, tx domain.Transaction) error {
	return f.postFn(ctx, tx)
}

func makePosting(t *testing.T, side domain.Side, currency money.Currency) domain.Posting {
	t.Helper()
	amount, err := money.New(100, currency)
	if err != nil {
		t.Fatal(err)
	}
	posting, err := domain.NewPosting(uuid.New(), side, amount)
	if err != nil {
		t.Fatal(err)
	}
	return posting
}

func TestPostTransaction(t *testing.T) {
	postings := []domain.Posting{makePosting(t, domain.Debit, money.CurrencyEUR), makePosting(t, domain.Credit, money.CurrencyEUR)}
	calls := 0
	var persisted domain.Transaction
	service := New(fakeRepository{postFn: func(ctx context.Context, tx domain.Transaction) error {
		calls++
		if ctx != t.Context() {
			t.Error("expected caller context to reach repository")
		}
		if err := tx.Validate(); err != nil {
			t.Errorf("invalid transaction reached repository: %v", err)
		}
		persisted = tx
		return nil
	}})
	tx, err := service.PostTransaction(t.Context(), postings)
	if err != nil {
		t.Fatalf("post transaction: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected one repository call, got %d", calls)
	}
	if tx.ID() == uuid.Nil || tx.ID() != persisted.ID() || !tx.CreatedAt().Equal(persisted.CreatedAt()) || !slices.Equal(tx.Postings(), persisted.Postings()) {
		t.Error("returned transaction differs from persisted transaction")
	}
}

func TestPostTransactionRejectsInvalidBeforePersistence(t *testing.T) {
	debit := makePosting(t, domain.Debit, money.CurrencyEUR)
	for _, tc := range []struct {
		name     string
		postings []domain.Posting
		err      error
	}{
		{"empty", nil, domain.ErrInvalidTransaction},
		{"one posting", []domain.Posting{debit}, domain.ErrInvalidTransaction},
		{"invalid posting", []domain.Posting{debit, {}}, domain.ErrInvalidPosting},
		{"duplicate posting", []domain.Posting{debit, debit}, domain.ErrInvalidTransaction},
		{"unbalanced", []domain.Posting{debit, makePosting(t, domain.Debit, money.CurrencyEUR)}, domain.ErrUnbalanced},
		{"cross currency", []domain.Posting{debit, makePosting(t, domain.Credit, money.CurrencyUSD)}, domain.ErrUnbalanced},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := New(fakeRepository{postFn: func(context.Context, domain.Transaction) error {
				t.Fatal("invalid transaction reached persistence")
				return nil
			}})
			tx, err := service.PostTransaction(t.Context(), tc.postings)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
			if tx.ID() != uuid.Nil {
				t.Error("expected no transaction on failure")
			}
		})
	}
}

func TestPostTransactionReturnsPersistenceError(t *testing.T) {
	postings := []domain.Posting{makePosting(t, domain.Debit, money.CurrencyEUR), makePosting(t, domain.Credit, money.CurrencyEUR)}
	for _, persistenceErr := range []error{errors.New("persistence failed"), context.Canceled} {
		t.Run(persistenceErr.Error(), func(t *testing.T) {
			service := New(fakeRepository{postFn: func(context.Context, domain.Transaction) error { return persistenceErr }})
			tx, err := service.PostTransaction(t.Context(), postings)
			if !errors.Is(err, persistenceErr) {
				t.Fatalf("expected %v, got %v", persistenceErr, err)
			}
			if tx.ID() != uuid.Nil || len(tx.Postings()) != 0 {
				t.Error("expected no transaction when persistence fails")
			}
		})
	}
}
