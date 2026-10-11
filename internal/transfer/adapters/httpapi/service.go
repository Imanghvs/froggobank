package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/transfer/domain"
)

type TransferService interface {
	Transfer(
		ctx context.Context,
		callerID uuid.UUID,
		source uuid.UUID,
		destination uuid.UUID,
		amountMinor string,
	) (domain.Transfer, error)
}
