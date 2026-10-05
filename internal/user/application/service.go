package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/user/domain"
)

type Repository interface {
	// FindOrCreate resolves (issuer, subject) atomically. A concurrent existing
	// user wins over the candidate's generated ID and timestamp.
	FindOrCreate(ctx context.Context, candidate domain.User) (domain.User, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) ResolveIdentity(ctx context.Context, identity domain.Identity) (domain.User, error) {
	candidate, err := domain.New(identity)
	if err != nil {
		return domain.User{}, err
	}
	return s.repository.FindOrCreate(ctx, candidate)
}
