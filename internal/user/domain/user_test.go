package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIdentityAndUserConstruction(t *testing.T) {
	identity, err := NewIdentity("https://issuer.example", "subject-123")
	if err != nil {
		t.Fatal(err)
	}
	user, err := New(identity)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == uuid.Nil || user.Identity != identity || identity.Issuer() != "https://issuer.example" || identity.Subject() != "subject-123" || user.CreatedAt.IsZero() || user.CreatedAt.Location() != time.UTC || user.CreatedAt.Nanosecond()%1000 != 0 {
		t.Fatalf("invalid user: %+v", user)
	}
}
func TestInvalidIdentities(t *testing.T) {
	for _, tc := range []struct{ issuer, subject string }{
		{"", "subject"}, {"https://issuer.example", ""}, {" https://issuer.example", "subject"},
		{"https://issuer.example", " "}, {strings.Repeat("x", 2049), "subject"}, {"https://issuer.example", strings.Repeat("x", 256)},
	} {
		if _, err := NewIdentity(tc.issuer, tc.subject); !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("invalid identity accepted: %v", err)
		}
	}
	if _, err := New(Identity{}); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatal("zero identity accepted")
	}
}
