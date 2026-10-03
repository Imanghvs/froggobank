package account

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseCurrencyReturnsErrorOnInvalidInput(t *testing.T) {
	if _, err := ParseCurrency("invalid"); err == nil {
		t.Fatal("error expected")
	}
}

func TestParseCurrencyReturnsTheRightCurrency(t *testing.T) {
	currency, err := ParseCurrency("EUR")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	expectedCurrency := Currency("EUR")
	if currency != expectedCurrency {
		t.Errorf("expected currency %v, received %v", expectedCurrency, currency)
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
}
