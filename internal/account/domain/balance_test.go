package domain

import (
	"errors"
	"math/big"
	"testing"

	"github.com/google/uuid"
)

func TestAccountTypesAndBalanceSigns(t *testing.T) {
	for _, tc := range []struct {
		accountType AccountType
		side        NormalSide
		posted      string
	}{
		{Asset, DebitNormal, "-75"},
		{Expense, DebitNormal, "-75"},
		{Liability, CreditNormal, "75"},
		{Equity, CreditNormal, "75"},
		{Revenue, CreditNormal, "75"},
	} {
		t.Run(string(tc.accountType), func(t *testing.T) {
			parsed, err := ParseAccountType(string(tc.accountType))
			if err != nil || parsed != tc.accountType {
				t.Fatalf("parse account type: %q, %v", parsed, err)
			}
			side, err := parsed.NormalSide()
			if err != nil || side != tc.side {
				t.Fatalf("normal side: %q, %v", side, err)
			}
			acc, err := NewWithType(CurrencyEUR, parsed)
			if err != nil {
				t.Fatal(err)
			}
			balance, err := NewBalance(acc, big.NewInt(25), big.NewInt(100))
			if err != nil {
				t.Fatal(err)
			}
			if balance.PostedMinor().String() != tc.posted {
				t.Errorf("expected %s, got %s", tc.posted, balance.PostedMinor())
			}
			if balance.AccountID() != acc.ID || balance.Currency() != acc.Currency || balance.AccountType() != acc.Type {
				t.Error("balance lost account identity")
			}
		})
	}
}

func TestAccountTypeValidation(t *testing.T) {
	for _, value := range []string{"", "other", "ASSET", " liability ", "contra_asset"} {
		if _, err := ParseAccountType(value); !errors.Is(err, ErrInvalidAccountType) {
			t.Errorf("expected invalid account type for %q, got %v", value, err)
		}
		if _, err := NewWithType(CurrencyEUR, AccountType(value)); !errors.Is(err, ErrInvalidAccountType) {
			t.Errorf("constructor accepted %q: %v", value, err)
		}
	}
	if _, err := NewWithType("JPY", Asset); !errors.Is(err, ErrInvalidCurrency) {
		t.Errorf("constructor accepted unsupported currency: %v", err)
	}
}

func TestBalanceUsesExactImmutableTotals(t *testing.T) {
	credits, _ := new(big.Int).SetString("18446744073709551614", 10)
	debits := big.NewInt(14)
	balance, err := NewBalance(New(CurrencyEUR), debits, credits)
	if err != nil {
		t.Fatal(err)
	}
	credits.SetInt64(0)
	debits.SetInt64(0)
	balance.DebitsMinor().SetInt64(0)
	balance.CreditsMinor().SetInt64(0)
	balance.PostedMinor().SetInt64(0)
	if balance.PostedMinor().String() != "18446744073709551600" || balance.DebitsMinor().String() != "14" {
		t.Fatalf("balance lost precision or was mutated: %s", balance.PostedMinor())
	}
}

func TestBalanceRejectsInvalidState(t *testing.T) {
	acc := New(CurrencyEUR)
	for _, tc := range []struct {
		name            string
		account         Account
		debits, credits *big.Int
		want            error
	}{
		{"nil debits", acc, nil, big.NewInt(0), ErrInvalidBalance},
		{"nil credits", acc, big.NewInt(0), nil, ErrInvalidBalance},
		{"negative debits", acc, big.NewInt(-1), big.NewInt(0), ErrInvalidBalance},
		{"negative credits", acc, big.NewInt(0), big.NewInt(-1), ErrInvalidBalance},
		{"missing account", Account{Currency: CurrencyEUR, Type: Asset}, big.NewInt(0), big.NewInt(0), ErrInvalidBalance},
		{"invalid currency", Account{ID: uuid.New(), Currency: "JPY", Type: Asset}, big.NewInt(0), big.NewInt(0), ErrInvalidCurrency},
		{"invalid type", Account{ID: uuid.New(), Currency: CurrencyEUR}, big.NewInt(0), big.NewInt(0), ErrInvalidAccountType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewBalance(tc.account, tc.debits, tc.credits); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}
