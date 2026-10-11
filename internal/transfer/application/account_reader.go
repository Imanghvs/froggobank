package application

import (
	"context"

	"github.com/google/uuid"

	account "github.com/Imanghvs/froggobank/internal/account/domain"
)

type AccountReader interface {
	GetAccountByID(ctx context.Context, callerID, accountID uuid.UUID) (account.Account, error)
}
