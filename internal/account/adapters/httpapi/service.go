package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/domain"
)

type AccountService interface {
	CreateAccount(ctx context.Context, callerID uuid.UUID, currencyCode string) (domain.Account, error)
	GetAccountByID(ctx context.Context, callerID, id uuid.UUID) (domain.Account, error)
	GetAccountBalance(ctx context.Context, callerID, id uuid.UUID) (domain.Balance, error)
	GetUserAccounts(ctx context.Context, callerID uuid.UUID, limit int, offset int) (domain.PaginatedAccountsResponse, error)
}
