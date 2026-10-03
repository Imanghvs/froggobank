package account

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("account not found")

type Repository interface {
	Create(ctx context.Context, acc Account) error
	GetByID(ctx context.Context, id uuid.UUID) (Account, error)
}
