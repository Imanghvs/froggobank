package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/transfer/domain"
)

type Repository interface {
	Create(
		ctx context.Context,
		tr domain.Transfer,
	) error
}
