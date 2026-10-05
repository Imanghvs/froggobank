package domain

import (
	"fmt"

	"github.com/google/uuid"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
)

type Side string

const (
	Debit  Side = "debit"
	Credit Side = "credit"
)

// Posting is an immutable debit or credit of positive minor units to an account.
type Posting struct {
	id        uuid.UUID
	accountID uuid.UUID
	side      Side
	amount    money.Money
}

func NewPosting(accountID uuid.UUID, side Side, amount money.Money) (Posting, error) {
	p := Posting{id: uuid.New(), accountID: accountID, side: side, amount: amount}
	if err := p.Validate(); err != nil {
		return Posting{}, err
	}
	return p, nil
}

func (p Posting) ID() uuid.UUID        { return p.id }
func (p Posting) AccountID() uuid.UUID { return p.accountID }
func (p Posting) Side() Side           { return p.side }
func (p Posting) Amount() money.Money  { return p.amount }

func (p Posting) Validate() error {
	if p.id == uuid.Nil || p.accountID == uuid.Nil {
		return fmt.Errorf("%w: posting and account IDs must be nonzero", ErrInvalidPosting)
	}
	if p.side != Debit && p.side != Credit {
		return fmt.Errorf("%w: unknown side %q", ErrInvalidPosting, p.side)
	}
	if err := p.amount.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPosting, err)
	}
	if p.amount.MinorUnits() <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidPosting)
	}
	return nil
}
