package domain

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
	"github.com/google/uuid"
)

func TestTransactionBalancesEachCurrency(t *testing.T) {
	var postings []Posting
	for _, currency := range []money.Currency{money.CurrencyEUR, money.CurrencyUSD, money.CurrencyGBP} {
		postings = append(postings, makePosting(t, Debit, 100, currency), makePosting(t, Credit, 100, currency))
	}
	tx, err := NewTransaction(postings)
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if tx.ID() == uuid.Nil || tx.CreatedAt().IsZero() || tx.CreatedAt().Location() != time.UTC || tx.CreatedAt().Nanosecond()%1000 != 0 {
		t.Error("expected generated ID and UTC timestamp with microsecond precision")
	}
	if !slices.Equal(tx.Postings(), postings) {
		t.Error("transaction changed postings")
	}
}

func TestTransactionBalancesTotalsBeyondInt64(t *testing.T) {
	postings := []Posting{
		makePosting(t, Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, Debit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, Credit, math.MaxInt64, money.CurrencyEUR),
		makePosting(t, Credit, math.MaxInt64, money.CurrencyEUR),
	}
	if _, err := NewTransaction(postings); err != nil {
		t.Fatalf("expected exact balanced totals, got %v", err)
	}
}

func TestTransactionRejectsInvalidOrUnbalancedPostings(t *testing.T) {
	debit := makePosting(t, Debit, 100, money.CurrencyEUR)
	credit := makePosting(t, Credit, 100, money.CurrencyEUR)
	duplicateID := credit
	duplicateID.id = debit.id
	for _, tc := range []struct {
		name     string
		postings []Posting
		err      error
	}{
		{"empty", nil, ErrInvalidTransaction},
		{"one posting", []Posting{debit}, ErrInvalidTransaction},
		{"zero-value posting", []Posting{debit, {}}, ErrInvalidPosting},
		{"duplicate ID", []Posting{debit, duplicateID}, ErrInvalidTransaction},
		{"unequal totals", []Posting{debit, makePosting(t, Credit, 99, money.CurrencyEUR)}, ErrUnbalanced},
		{"no credits", []Posting{debit, makePosting(t, Debit, 100, money.CurrencyEUR)}, ErrUnbalanced},
		{"no debits", []Posting{credit, makePosting(t, Credit, 100, money.CurrencyEUR)}, ErrUnbalanced},
		{"cross currency", []Posting{debit, makePosting(t, Credit, 100, money.CurrencyUSD)}, ErrUnbalanced},
		{"integer overflow cannot hide imbalance", []Posting{
			makePosting(t, Debit, math.MaxInt64, money.CurrencyEUR),
			makePosting(t, Debit, math.MaxInt64, money.CurrencyEUR),
			makePosting(t, Debit, 3, money.CurrencyEUR),
			makePosting(t, Credit, 1, money.CurrencyEUR),
		}, ErrUnbalanced},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := NewTransaction(tc.postings)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
			if tx.ID() != uuid.Nil || len(tx.Postings()) != 0 {
				t.Error("expected no transaction on failure")
			}
		})
	}
}

func TestTransactionIsImmutableThroughSlices(t *testing.T) {
	postings := []Posting{makePosting(t, Debit, 100, money.CurrencyEUR), makePosting(t, Credit, 100, money.CurrencyEUR)}
	expected := slices.Clone(postings)
	tx, err := NewTransaction(postings)
	if err != nil {
		t.Fatal(err)
	}
	postings[0] = Posting{}
	returned := tx.Postings()
	returned[1] = Posting{}
	if !slices.Equal(tx.Postings(), expected) {
		t.Error("caller modified transaction postings")
	}
	if err := tx.Validate(); err != nil {
		t.Errorf("transaction became invalid: %v", err)
	}
}

func TestTransactionRequiresMetadata(t *testing.T) {
	if err := (Transaction{}).Validate(); !errors.Is(err, ErrInvalidTransaction) {
		t.Errorf("expected invalid zero-value transaction, got %v", err)
	}
	postings := []Posting{makePosting(t, Debit, 1, money.CurrencyEUR), makePosting(t, Credit, 1, money.CurrencyEUR)}
	tx, err := NewTransaction(postings)
	if err != nil {
		t.Fatal(err)
	}
	missingID := tx
	missingID.id = uuid.Nil
	missingTime := tx
	missingTime.createdAt = time.Time{}
	for _, invalid := range []Transaction{missingID, missingTime} {
		if err := invalid.Validate(); !errors.Is(err, ErrInvalidTransaction) {
			t.Errorf("expected invalid metadata error, got %v", err)
		}
	}
}
