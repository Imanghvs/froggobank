package domain

import (
	"math/big"

	"github.com/google/uuid"
)

// Balance is a snapshot of cumulative posted totals. It uses exact integers
// because lifetime turnover can exceed the int64 capacity of one posting.
type Balance struct {
	accountID   uuid.UUID
	currency    Currency
	accountType AccountType
	debits      big.Int
	credits     big.Int
}

func NewBalance(account Account, debits, credits *big.Int) (Balance, error) {
	if account.ID == uuid.Nil || debits == nil || credits == nil || debits.Sign() < 0 || credits.Sign() < 0 {
		return Balance{}, ErrInvalidBalance
	}
	if _, err := account.Currency.Scale(); err != nil {
		return Balance{}, err
	}
	if _, err := account.Type.NormalSide(); err != nil {
		return Balance{}, err
	}
	b := Balance{accountID: account.ID, currency: account.Currency, accountType: account.Type}
	b.debits.Set(debits)
	b.credits.Set(credits)
	return b, nil
}

func (b Balance) AccountID() uuid.UUID     { return b.accountID }
func (b Balance) Currency() Currency       { return b.currency }
func (b Balance) AccountType() AccountType { return b.accountType }
func (b Balance) DebitsMinor() *big.Int    { return new(big.Int).Set(&b.debits) }
func (b Balance) CreditsMinor() *big.Int   { return new(big.Int).Set(&b.credits) }

func (b Balance) PostedMinor() *big.Int {
	if b.accountType == Asset || b.accountType == Expense {
		return new(big.Int).Sub(&b.debits, &b.credits)
	}
	return new(big.Int).Sub(&b.credits, &b.debits)
}
