package application

import "github.com/Imanghvs/froggobank/internal/account/domain"

type AccountFilter struct {
	Limit    int
	Offset   int
	Currency *domain.Currency
}
