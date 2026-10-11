package domain

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
)

func TestNew(t *testing.T) {
	source, destination := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name     string
		currency money.Currency
		minor    int64
	}{
		{"EUR", money.CurrencyEUR, 2500},
		{"USD", money.CurrencyUSD, 1},
		{"GBP", money.CurrencyGBP, 100},
		{"maximum amount", money.CurrencyEUR, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amount, err := money.New(tc.minor, tc.currency)
			if err != nil {
				t.Fatal(err)
			}
			before := time.Now().UTC().Truncate(time.Microsecond)
			tr, err := New(source, destination, amount)
			if err != nil {
				t.Fatalf("create transfer: %v", err)
			}
			if tr.ID == uuid.Nil || tr.Source != source || tr.Destination != destination ||
				tr.AmountMinor != amount || tr.Currency != amount.Currency() {
				t.Fatalf("unexpected transfer: %+v", tr)
			}
			if tr.CreatedAt.Before(before) || tr.CreatedAt.After(time.Now().UTC()) ||
				tr.CreatedAt.Location() != time.UTC || tr.CreatedAt.Nanosecond()%1000 != 0 {
				t.Fatalf("expected a current UTC timestamp with microsecond precision, got %s", tr.CreatedAt)
			}
		})
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	source, destination := uuid.New(), uuid.New()
	amount := func(minor int64) money.Money {
		t.Helper()
		value, err := money.New(minor, money.CurrencyEUR)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, tc := range []struct {
		name                string
		source, destination uuid.UUID
		amount              money.Money
		wantErr             error
	}{
		{"missing source", uuid.Nil, destination, amount(2500), ErrInvalidAccountID},
		{"missing destination", source, uuid.Nil, amount(2500), ErrInvalidAccountID},
		{"both accounts missing", uuid.Nil, uuid.Nil, amount(2500), ErrInvalidAccountID},
		{"identical accounts", source, source, amount(2500), ErrSameAccount},
		{"uninitialized money", source, destination, money.Money{}, ErrInvalidCurrency},
		{"zero amount", source, destination, amount(0), ErrInvalidAmount},
		{"negative amount", source, destination, amount(-1), ErrInvalidAmount},
		{"minimum amount", source, destination, amount(math.MinInt64), ErrInvalidAmount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := New(tc.source, tc.destination, tc.amount)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tr != (Transfer{}) {
				t.Fatalf("failed construction returned a transfer: %+v", tr)
			}
		})
	}
}
