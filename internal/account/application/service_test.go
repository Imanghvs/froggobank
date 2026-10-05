package application

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeRepository struct {
	createFn     func(context.Context, domain.Account) error
	getByIDFn    func(context.Context, uuid.UUID) (domain.Account, error)
	getBalanceFn func(context.Context, uuid.UUID) (domain.Balance, error)
}

func (f fakeRepository) Create(ctx context.Context, a domain.Account) error {
	return f.createFn(ctx, a)
}
func (f fakeRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return f.getByIDFn(ctx, id)
}
func (f fakeRepository) GetBalance(ctx context.Context, id uuid.UUID) (domain.Balance, error) {
	return f.getBalanceFn(ctx, id)
}

func TestCreateCustomerAccount(t *testing.T) {
	caller := uuid.New()
	for _, currency := range []string{"EUR", "USD", "GBP"} {
		t.Run(currency, func(t *testing.T) {
			var persisted domain.Account
			service := New(fakeRepository{createFn: func(ctx context.Context, a domain.Account) error {
				if ctx != t.Context() {
					t.Error("caller context lost")
				}
				persisted = a
				return nil
			}})
			a, err := service.CreateAccount(t.Context(), caller, currency)
			if err != nil {
				t.Fatal(err)
			}
			if a != persisted || a.ID == uuid.Nil || a.OwnerID != caller || a.Type != domain.Liability || !a.EnforceNonnegativeBalance || string(a.Currency) != currency {
				t.Fatalf("incorrect customer account: %+v", a)
			}
			if a.CreatedAt.IsZero() || a.CreatedAt.Location() != time.UTC || a.CreatedAt.Nanosecond()%1000 != 0 {
				t.Error("invalid timestamp")
			}
		})
	}
}
func TestCreateRejectsInvalidCurrencyBeforePersistence(t *testing.T) {
	for _, code := range []string{"", "JPY", "eur", " EUR "} {
		_, err := New(nil).CreateAccount(t.Context(), uuid.New(), code)
		if !errors.Is(err, domain.ErrInvalidCurrency) {
			t.Errorf("currency %q: %v", code, err)
		}
	}
}
func TestUnauthenticatedApplicationCallsDoNotReachRepository(t *testing.T) {
	s := New(nil)
	if _, err := s.CreateAccount(t.Context(), uuid.Nil, "EUR"); !errors.Is(err, user.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if _, err := s.GetAccountByID(t.Context(), uuid.Nil, uuid.New()); !errors.Is(err, user.ErrUnauthenticated) {
		t.Fatal(err)
	}
	if _, err := s.GetAccountBalance(t.Context(), uuid.Nil, uuid.New()); !errors.Is(err, user.ErrUnauthenticated) {
		t.Fatal(err)
	}
}
func TestApplicationOwnershipChecks(t *testing.T) {
	caller := uuid.New()
	own, err := domain.NewCustomer(domain.CurrencyEUR, caller)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		modify  func(*domain.Account)
		allowed bool
	}{
		{"own", func(*domain.Account) {}, true},
		{"another user", func(a *domain.Account) { a.OwnerID = uuid.New() }, false},
		{"internal", func(a *domain.Account) { a.OwnerID = uuid.Nil }, false},
		{"system accounting type", func(a *domain.Account) { a.Type = domain.Asset }, false},
		{"unrestricted", func(a *domain.Account) { a.EnforceNonnegativeBalance = false }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := own
			tc.modify(&a)
			balanceReads := 0
			s := New(fakeRepository{getByIDFn: func(ctx context.Context, id uuid.UUID) (domain.Account, error) {
				if ctx != t.Context() || id != a.ID {
					t.Error("lookup context or ID lost")
				}
				return a, nil
			}, getBalanceFn: func(context.Context, uuid.UUID) (domain.Balance, error) {
				balanceReads++
				return domain.NewBalance(a, big.NewInt(2500), big.NewInt(10000))
			}})
			got, err := s.GetAccountByID(t.Context(), caller, a.ID)
			b, balanceErr := s.GetAccountBalance(t.Context(), caller, a.ID)
			if tc.allowed {
				if err != nil || balanceErr != nil || got != a || b.PostedMinor().String() != "7500" || balanceReads != 1 {
					t.Fatalf("own-account access failed: %v %v", err, balanceErr)
				}
			} else if !errors.Is(err, domain.ErrNotFound) || !errors.Is(balanceErr, domain.ErrNotFound) || balanceReads != 0 {
				t.Fatalf("unauthorized data exposed: %v %v reads=%d", err, balanceErr, balanceReads)
			}
		})
	}
}
func TestServicePreservesRepositoryErrors(t *testing.T) {
	caller := uuid.New()
	a, _ := domain.NewCustomer(domain.CurrencyEUR, caller)
	for _, want := range []error{fmt.Errorf("lookup: %w", domain.ErrNotFound), context.Canceled, errors.New("database unavailable")} {
		s := New(fakeRepository{
			createFn:  func(context.Context, domain.Account) error { return want },
			getByIDFn: func(context.Context, uuid.UUID) (domain.Account, error) { return domain.Account{}, want },
		})
		if _, err := s.CreateAccount(t.Context(), caller, "EUR"); !errors.Is(err, want) {
			t.Fatal(err)
		}
		if _, err := s.GetAccountByID(t.Context(), caller, a.ID); !errors.Is(err, want) {
			t.Fatal(err)
		}
		if _, err := s.GetAccountBalance(t.Context(), caller, a.ID); !errors.Is(err, want) {
			t.Fatal(err)
		}
		s = New(fakeRepository{
			getByIDFn:    func(context.Context, uuid.UUID) (domain.Account, error) { return a, nil },
			getBalanceFn: func(context.Context, uuid.UUID) (domain.Balance, error) { return domain.Balance{}, want },
		})
		if _, err := s.GetAccountBalance(t.Context(), caller, a.ID); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
}
