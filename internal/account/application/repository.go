package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/google/uuid"
)

// Repository is the persistence capability required by the account use cases.
type Repository interface {
	Create(ctx context.Context, acc domain.Account) error
	// GetByID returns domain.ErrNotFound when the account does not exist.
	GetByID(ctx context.Context, id uuid.UUID) (domain.Account, error)
}
