package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/ledger/domain"
)

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) PostTransaction(ctx context.Context, postings []domain.Posting) (domain.Transaction, error) {
	transaction, err := domain.NewTransaction(postings)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := s.repository.Post(ctx, transaction); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}
