package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	account "github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/Imanghvs/froggobank/internal/transfer/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeRepository struct {
	createFn func(context.Context, domain.Transfer) error
}

func (f fakeRepository) Create(ctx context.Context, tr domain.Transfer) error {
	return f.createFn(ctx, tr)
}

type fakeAccountReader func(context.Context, uuid.UUID, uuid.UUID) (account.Account, error)

func (f fakeAccountReader) GetAccountByID(ctx context.Context, callerID, id uuid.UUID) (account.Account, error) {
	return f(ctx, callerID, id)
}

func TestTransferRejectsInvalidInputBeforeAccountReads(t *testing.T) {
	caller, source, destination := uuid.New(), uuid.New(), uuid.New()
	for _, tc := range []struct {
		name                        string
		caller, source, destination uuid.UUID
		amount                      string
		wantErr                     error
	}{
		{"unauthenticated", uuid.Nil, source, destination, "2500", user.ErrUnauthenticated},
		{"missing source", caller, uuid.Nil, destination, "2500", domain.ErrInvalidAccountID},
		{"missing destination", caller, source, uuid.Nil, "2500", domain.ErrInvalidAccountID},
		{"identical accounts", caller, source, source, "2500", domain.ErrSameAccount},
		{"empty amount", caller, source, destination, "", domain.ErrInvalidAmount},
		{"zero amount", caller, source, destination, "0", domain.ErrInvalidAmount},
		{"negative amount", caller, source, destination, "-1", domain.ErrInvalidAmount},
		{"fractional amount", caller, source, destination, "1.5", domain.ErrInvalidAmount},
		{"leading plus", caller, source, destination, "+2500", domain.ErrInvalidAmount},
		{"leading zero", caller, source, destination, "02500", domain.ErrInvalidAmount},
		{"overflow", caller, source, destination, "9223372036854775808", domain.ErrInvalidAmount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(fakeRepository{}, fakeAccountReader(func(context.Context, uuid.UUID, uuid.UUID) (account.Account, error) {
				t.Fatal("invalid input reached account reader")
				return account.Account{}, nil
			}))
			_, err := s.Transfer(t.Context(), tc.caller, tc.source, tc.destination, tc.amount)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestTransferClassifiesOnlyAccountNotFoundErrors(t *testing.T) {
	caller, source, destination := uuid.New(), uuid.New(), uuid.New()
	databaseErr := errors.New("private database connection details")
	for _, tc := range []struct {
		name      string
		failedID  uuid.UUID
		readErr   error
		wantErr   error
		wantReads int
	}{
		{"missing source", source, fmt.Errorf("lookup: %w", account.ErrNotFound), domain.ErrSourceNotFound, 1},
		{"missing destination", destination, fmt.Errorf("lookup: %w", account.ErrNotFound), domain.ErrDestinationNotFound, 2},
		{"source database failure", source, databaseErr, databaseErr, 1},
		{"destination database failure", destination, databaseErr, databaseErr, 2},
		{"source unauthenticated", source, user.ErrUnauthenticated, user.ErrUnauthenticated, 1},
		{"destination unauthenticated", destination, user.ErrUnauthenticated, user.ErrUnauthenticated, 2},
		{"canceled lookup", source, context.Canceled, context.Canceled, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			s := New(fakeRepository{}, fakeAccountReader(func(_ context.Context, id, accountID uuid.UUID) (account.Account, error) {
				reads++
				if id != caller {
					t.Fatal("account lookup lost authenticated caller")
				}
				if accountID == tc.failedID {
					return account.Account{}, tc.readErr
				}
				return account.Account{ID: accountID, Currency: account.CurrencyEUR}, nil
			}))
			_, err := s.Transfer(t.Context(), caller, source, destination, "2500")
			if !errors.Is(err, tc.wantErr) || reads != tc.wantReads {
				t.Fatalf("error = %v, reads = %d; want %v, %d", err, reads, tc.wantErr, tc.wantReads)
			}
		})
	}
}

func TestTransferCurrencyAndPersistenceErrors(t *testing.T) {
	caller, source, destination := uuid.New(), uuid.New(), uuid.New()
	persistenceErr := errors.New("private insert details")
	for _, tc := range []struct {
		name                                string
		sourceCurrency, destinationCurrency account.Currency
		repoErr, wantErr                    error
		wantCreates                         int
	}{
		{"source currency mismatch", account.CurrencyUSD, account.CurrencyEUR, nil, domain.ErrCurrencyMismatch, 0},
		{"destination currency mismatch", account.CurrencyEUR, account.CurrencyUSD, nil, domain.ErrCurrencyMismatch, 0},
		{"USD currency derived from accounts", account.CurrencyUSD, account.CurrencyUSD, nil, nil, 1},
		{"GBP currency derived from accounts", account.CurrencyGBP, account.CurrencyGBP, nil, nil, 1},
		{"persistence failure", account.CurrencyEUR, account.CurrencyEUR, persistenceErr, persistenceErr, 1},
		{"success", account.CurrencyEUR, account.CurrencyEUR, nil, nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			creates := 0
			s := New(fakeRepository{createFn: func(_ context.Context, tr domain.Transfer) error {
				creates++
				if tr.ID == uuid.Nil || tr.Source != source || tr.Destination != destination || tr.AmountMinor.MinorUnits() != 2500 || tr.AmountMinor.Currency() != tc.sourceCurrency || tr.Currency != tc.sourceCurrency {
					t.Fatalf("invalid transfer reached persistence: %+v", tr)
				}
				return tc.repoErr
			}}, fakeAccountReader(func(_ context.Context, _ uuid.UUID, id uuid.UUID) (account.Account, error) {
				currency := tc.sourceCurrency
				if id == destination {
					currency = tc.destinationCurrency
				}
				return account.Account{ID: id, Currency: currency}, nil
			}))
			tr, err := s.Transfer(t.Context(), caller, source, destination, "2500")
			if !errors.Is(err, tc.wantErr) || creates != tc.wantCreates {
				t.Fatalf("error = %v, creates = %d; want %v, %d", err, creates, tc.wantErr, tc.wantCreates)
			}
			if err == nil && tr.ID == uuid.Nil {
				t.Fatal("successful transfer has no ID")
			}
		})
	}
}
