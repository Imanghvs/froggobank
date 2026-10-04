package domain

import (
	"fmt"
	"math/big"
	"slices"
	"time"

	money "github.com/Imanghvs/froggobank/internal/money/domain"
	"github.com/google/uuid"
)

// Transaction is an immutable set of postings balanced separately per currency.
type Transaction struct {
	id        uuid.UUID
	createdAt time.Time
	postings  []Posting
}

func NewTransaction(postings []Posting) (Transaction, error) {
	tx := Transaction{
		id:        uuid.New(),
		createdAt: time.Now().UTC().Truncate(time.Microsecond),
		postings:  slices.Clone(postings),
	}
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

func (t Transaction) ID() uuid.UUID        { return t.id }
func (t Transaction) CreatedAt() time.Time { return t.createdAt }
func (t Transaction) Postings() []Posting  { return slices.Clone(t.postings) }

func (t Transaction) Validate() error {
	if t.id == uuid.Nil || t.createdAt.IsZero() {
		return fmt.Errorf("%w: ID and creation time are required", ErrInvalidTransaction)
	}
	if len(t.postings) < 2 {
		return fmt.Errorf("%w: at least two postings are required", ErrInvalidTransaction)
	}
	seen := make(map[uuid.UUID]struct{}, len(t.postings))
	net := make(map[money.Currency]*big.Int)
	for _, posting := range t.postings {
		if err := posting.Validate(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTransaction, err)
		}
		if _, duplicate := seen[posting.id]; duplicate {
			return fmt.Errorf("%w: duplicate posting ID %s", ErrInvalidTransaction, posting.id)
		}
		seen[posting.id] = struct{}{}
		currency := posting.amount.Currency()
		if net[currency] == nil {
			net[currency] = new(big.Int)
		}
		// Totals may exceed int64 even though each money value fits. Accumulate
		// exactly so overflow cannot make an unbalanced transaction appear valid.
		amount := big.NewInt(posting.amount.MinorUnits())
		if posting.side == Debit {
			net[currency].Add(net[currency], amount)
		} else {
			net[currency].Sub(net[currency], amount)
		}
	}
	for currency, total := range net {
		if total.Sign() != 0 {
			return fmt.Errorf("%w: %s", ErrUnbalanced, currency)
		}
	}
	return nil
}
