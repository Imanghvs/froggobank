package domain

import (
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
)

func makePosting(t *testing.T, side Side, units int64, currency money.Currency) Posting {
	t.Helper()
	amount, err := money.New(units, currency)
	if err != nil {
		t.Fatal(err)
	}
	posting, err := NewPosting(uuid.New(), side, amount)
	if err != nil {
		t.Fatal(err)
	}
	return posting
}

func TestNewPosting(t *testing.T) {
	for _, side := range []Side{Debit, Credit} {
		for _, units := range []int64{1, math.MaxInt64} {
			accountID := uuid.New()
			amount, err := money.New(units, money.CurrencyEUR)
			if err != nil {
				t.Fatal(err)
			}
			posting, err := NewPosting(accountID, side, amount)
			if err != nil {
				t.Fatalf("create posting: %v", err)
			}
			if posting.ID() == uuid.Nil || posting.AccountID() != accountID || posting.Side() != side || posting.Amount() != amount {
				t.Errorf("unexpected posting: %+v", posting)
			}
		}
	}
}

func TestPostingRejectsInvalidStructure(t *testing.T) {
	positive, _ := money.New(1, money.CurrencyEUR)
	zero, _ := money.New(0, money.CurrencyEUR)
	negative, _ := money.New(-1, money.CurrencyEUR)
	minimum, _ := money.New(math.MinInt64, money.CurrencyEUR)
	for _, tc := range []struct {
		name      string
		accountID uuid.UUID
		side      Side
		amount    money.Money
	}{
		{"missing account", uuid.Nil, Debit, positive},
		{"missing side", uuid.New(), "", positive},
		{"invalid side", uuid.New(), "other", positive},
		{"zero", uuid.New(), Debit, zero},
		{"negative", uuid.New(), Credit, negative},
		{"minimum", uuid.New(), Credit, minimum},
		{"invalid money", uuid.New(), Debit, money.Money{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posting, err := NewPosting(tc.accountID, tc.side, tc.amount)
			if !errors.Is(err, ErrInvalidPosting) {
				t.Fatalf("expected ErrInvalidPosting, got %v", err)
			}
			if posting != (Posting{}) {
				t.Error("expected no posting on failure")
			}
		})
	}
	if err := (Posting{}).Validate(); !errors.Is(err, ErrInvalidPosting) {
		t.Errorf("expected invalid zero-value posting, got %v", err)
	}
	posting := makePosting(t, Debit, 1, money.CurrencyEUR)
	posting.id = uuid.Nil
	if err := posting.Validate(); !errors.Is(err, ErrInvalidPosting) {
		t.Errorf("expected nonzero posting ID to be required, got %v", err)
	}
}
