package application

import (
	"context"

	"github.com/Imanghvs/froggobank/internal/user/domain"
	"github.com/google/uuid"
)

type userContextKey struct{}

// WithUser is used by trusted authentication adapters after verification and
// local-user resolution. Application account use cases also require the caller
// explicitly, so their ownership checks do not depend on HTTP middleware.
func WithUser(ctx context.Context, user domain.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func UserFromContext(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(domain.User)
	return user, ok && user.ID != uuid.Nil
}
