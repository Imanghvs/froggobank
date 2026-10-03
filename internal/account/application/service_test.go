package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/google/uuid"
)

type fakeRepository struct {
	createFn  func(context.Context, domain.Account) error
	getByIDFn func(context.Context, uuid.UUID) (domain.Account, error)
}

func (f fakeRepository) Create(ctx context.Context, acc domain.Account) error {
	return f.createFn(ctx, acc)
}

func (f fakeRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return f.getByIDFn(ctx, id)
}

func TestServiceCreateAccount(t *testing.T) {
	for _, currency := range []domain.Currency{domain.CurrencyEUR, domain.CurrencyUSD, domain.CurrencyGBP} {
		t.Run(string(currency), func(t *testing.T) {
			ctx := t.Context()
			var persisted domain.Account
			calls := 0
			service := New(fakeRepository{
				createFn: func(receivedCtx context.Context, acc domain.Account) error {
					calls++
					if receivedCtx != ctx {
						t.Error("expected caller context to reach repository")
					}
					persisted = acc
					return nil
				},
			})

			acc, err := service.CreateAccount(ctx, string(currency))
			if err != nil {
				t.Fatalf("create account: %v", err)
			}
			if calls != 1 {
				t.Fatalf("expected one repository call, got %d", calls)
			}
			if acc != persisted {
				t.Error("expected the returned account to match the persisted account")
			}
			if acc.ID == uuid.Nil {
				t.Error("expected application-generated account ID")
			}
			if acc.Currency != currency {
				t.Errorf("expected currency %q, got %q", currency, acc.Currency)
			}
			if acc.CreatedAt.IsZero() || acc.CreatedAt.Location() != time.UTC || acc.CreatedAt.Nanosecond()%1000 != 0 {
				t.Error("expected a UTC creation time with microsecond precision")
			}
		})
	}
}

func TestServiceCreateAccountRejectsUnsupportedCurrency(t *testing.T) {
	for _, currency := range []string{"JPY", "", "eur", " EUR "} {
		t.Run(currency, func(t *testing.T) {
			service := New(fakeRepository{
				createFn: func(context.Context, domain.Account) error {
					t.Fatal("repository must not be called for an unsupported currency")
					return nil
				},
			})

			acc, err := service.CreateAccount(t.Context(), currency)
			if !errors.Is(err, domain.ErrInvalidCurrency) {
				t.Fatalf("expected ErrInvalidCurrency, got %v", err)
			}
			if acc != (domain.Account{}) {
				t.Error("expected no account on validation failure")
			}
		})
	}
}

func TestServiceCreateAccountReturnsRepositoryError(t *testing.T) {
	for _, repositoryErr := range []error{errors.New("persistence failed"), context.Canceled} {
		t.Run(repositoryErr.Error(), func(t *testing.T) {
			service := New(fakeRepository{
				createFn: func(context.Context, domain.Account) error {
					return repositoryErr
				},
			})

			acc, err := service.CreateAccount(t.Context(), "EUR")
			if !errors.Is(err, repositoryErr) {
				t.Fatalf("expected repository error %v, got %v", repositoryErr, err)
			}
			if acc != (domain.Account{}) {
				t.Error("expected no account when persistence fails")
			}
		})
	}
}

func TestServiceGetAccountByID(t *testing.T) {
	ctx := t.Context()
	expected := domain.New(domain.CurrencyEUR)
	calls := 0
	service := New(fakeRepository{
		getByIDFn: func(receivedCtx context.Context, id uuid.UUID) (domain.Account, error) {
			calls++
			if receivedCtx != ctx {
				t.Error("expected caller context to reach repository")
			}
			if id != expected.ID {
				t.Errorf("expected ID %s, got %s", expected.ID, id)
			}
			return expected, nil
		},
	})

	acc, err := service.GetAccountByID(ctx, expected.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one repository call, got %d", calls)
	}
	if acc != expected {
		t.Errorf("expected account %+v, got %+v", expected, acc)
	}
}

func TestServiceGetAccountByIDPreservesRepositoryErrors(t *testing.T) {
	for _, repositoryErr := range []error{
		fmt.Errorf("lookup: %w", domain.ErrNotFound),
		errors.New("persistence failed"),
		context.Canceled,
	} {
		t.Run(repositoryErr.Error(), func(t *testing.T) {
			service := New(fakeRepository{
				getByIDFn: func(context.Context, uuid.UUID) (domain.Account, error) {
					return domain.Account{}, repositoryErr
				},
			})

			acc, err := service.GetAccountByID(t.Context(), uuid.New())
			if !errors.Is(err, repositoryErr) {
				t.Fatalf("expected repository error %v, got %v", repositoryErr, err)
			}
			if errors.Is(repositoryErr, domain.ErrNotFound) && !errors.Is(err, domain.ErrNotFound) {
				t.Error("expected ErrNotFound semantics to be preserved")
			}
			if acc != (domain.Account{}) {
				t.Error("expected no account on lookup failure")
			}
		})
	}
}
