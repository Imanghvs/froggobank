package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseCurrencyReturnsErrorOnInvalidInput(t *testing.T) {
	for _, value := range []string{"invalid", "JPY", "", "eur", " EUR "} {
		t.Run(value, func(t *testing.T) {
			currency, err := ParseCurrency(value)
			if !errors.Is(err, ErrInvalidCurrency) {
				t.Fatalf("expected ErrInvalidCurrency, got %v", err)
			}
			if currency != "" {
				t.Errorf("expected empty currency on error, got %q", currency)
			}
		})
	}
}

func TestParseCurrencyReturnsTheRightCurrency(t *testing.T) {
	for _, expectedCurrency := range []Currency{CurrencyEUR, CurrencyUSD, CurrencyGBP} {
		t.Run(string(expectedCurrency), func(t *testing.T) {
			currency, err := ParseCurrency(string(expectedCurrency))
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if currency != expectedCurrency {
				t.Errorf("expected currency %v, received %v", expectedCurrency, currency)
			}
		})
	}
}

func TestAccountConstructor(t *testing.T) {
	account := New(CurrencyEUR)

	if account.ID == uuid.Nil {
		t.Error("id is not generated")
	}

	if account.Currency != CurrencyEUR {
		t.Errorf("expected currency %q, received %q", CurrencyEUR, account.Currency)
	}

	if account.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}

	if account.CreatedAt.Location() != time.UTC {
		t.Errorf("expected created_at to be UTC, got %s", account.CreatedAt.Location())
	}

	if account.CreatedAt.Nanosecond()%1000 != 0 {
		t.Error("expected created_at to have microsecond precision")
	}
}
