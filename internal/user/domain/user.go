package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrInvalidIdentity = errors.New("invalid identity")
)

// Identity is an issuer-scoped subject obtained from a verified access token.
// Email is deliberately not an identity key.
type Identity struct {
	issuer  string
	subject string
}

func NewIdentity(issuer, subject string) (Identity, error) {
	i := Identity{issuer: issuer, subject: subject}
	if err := i.Validate(); err != nil {
		return Identity{}, err
	}
	return i, nil
}

func (i Identity) Validate() error {
	if len(i.issuer) == 0 || len(i.issuer) > 2048 || strings.TrimSpace(i.issuer) != i.issuer ||
		len(i.subject) == 0 || len(i.subject) > 255 || strings.TrimSpace(i.subject) == "" {
		return ErrInvalidIdentity
	}
	return nil
}

func (i Identity) Issuer() string  { return i.issuer }
func (i Identity) Subject() string { return i.subject }

type User struct {
	ID        uuid.UUID
	Identity  Identity
	CreatedAt time.Time
}

func New(identity Identity) (User, error) {
	if err := identity.Validate(); err != nil {
		return User{}, err
	}
	return User{ID: uuid.New(), Identity: identity, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}, nil
}
