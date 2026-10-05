package application

import (
	"context"
	"errors"
	"testing"

	"github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeRepository struct {
	resolve func(context.Context, domain.User) (domain.User, error)
}

func (f fakeRepository) FindOrCreate(ctx context.Context, u domain.User) (domain.User, error) {
	return f.resolve(ctx, u)
}
func TestResolveIdentityUsesExistingUserAndCallerContext(t *testing.T) {
	identity, _ := domain.NewIdentity("https://issuer.example", "subject")
	existing, _ := domain.New(identity)
	s := New(fakeRepository{resolve: func(ctx context.Context, candidate domain.User) (domain.User, error) {
		if ctx != t.Context() || candidate.Identity != identity || candidate.ID == existing.ID {
			t.Error("invalid candidate or context")
		}
		return existing, nil
	}})
	got, err := s.ResolveIdentity(t.Context(), identity)
	if err != nil || got != existing {
		t.Fatalf("resolve: %+v %v", got, err)
	}
}
func TestResolveRejectsInvalidIdentityBeforePersistence(t *testing.T) {
	if _, err := New(nil).ResolveIdentity(t.Context(), domain.Identity{}); !errors.Is(err, domain.ErrInvalidIdentity) {
		t.Fatal(err)
	}
}
func TestResolvePreservesErrors(t *testing.T) {
	identity, _ := domain.NewIdentity("https://issuer.example", "subject")
	for _, want := range []error{context.Canceled, errors.New("database unavailable")} {
		s := New(fakeRepository{resolve: func(context.Context, domain.User) (domain.User, error) { return domain.User{}, want }})
		if _, err := s.ResolveIdentity(t.Context(), identity); !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
}
