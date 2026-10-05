package oidc

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Imanghvs/froggobank/internal/testutil/oidcfixture"
	"github.com/Imanghvs/froggobank/internal/user/domain"
)

func TestVerifyAccessTokensAndRejectOtherCredentials(t *testing.T) {
	p := oidcfixture.New(t)
	for _, profile := range []string{RFC9068, Keycloak} {
		t.Run(profile, func(t *testing.T) {
			v, err := New(t.Context(), Config{IssuerURL: p.Issuer, Audience: "froggobank-api", AccessTokenProfile: profile, AllowInsecureHTTP: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name   string
				claims map[string]any
				valid  bool
			}{
				{"valid", nil, true},
				{"expired", map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}, false},
				{"no expiry", map[string]any{"exp": nil}, false},
				{"wrong issuer", map[string]any{"iss": "https://attacker.example"}, false},
				{"wrong audience", map[string]any{"aud": []string{"other-api"}}, false},
				{"no audience", map[string]any{"aud": nil}, false},
				{"no subject", map[string]any{"sub": nil}, false},
				{"empty subject", map[string]any{"sub": ""}, false},
				{"not yet valid", map[string]any{"nbf": time.Now().Add(time.Minute).Unix()}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					raw := p.Token(t, profile, "subject", tc.claims)
					identity, err := v.Verify(t.Context(), raw)
					if tc.valid {
						if err != nil || identity.Issuer() != p.Issuer || identity.Subject() != "subject" {
							t.Fatalf("valid token: %v", err)
						}
					} else if !errors.Is(err, domain.ErrUnauthenticated) {
						t.Fatalf("invalid token accepted: %v", err)
					}
				})
			}
			raw := p.Token(t, profile, "subject", nil)
			parts := strings.Split(raw, ".")
			parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"attacker"}`))
			if _, err := v.Verify(t.Context(), strings.Join(parts, ".")); !errors.Is(err, domain.ErrUnauthenticated) {
				t.Fatal("tampered token accepted")
			}
			// ID tokens must fail even if incorrectly given the API audience.
			idToken := p.Sign(t, "JWT", map[string]any{"iss": p.Issuer, "sub": "subject", "aud": "froggobank-api", "exp": time.Now().Add(time.Minute).Unix(), "typ": "ID"})
			if _, err := v.Verify(t.Context(), idToken); !errors.Is(err, domain.ErrUnauthenticated) {
				t.Fatal("ID token accepted as access token")
			}
			for _, bad := range []string{"", "not-a-jwt", strings.Repeat("x", MaxTokenBytes+1),
				base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"at+jwt"}`)) + "." + parts[1] + ".",
				base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"at+jwt"}`)) + "." + parts[1] + ".invalid",
			} {
				if _, err := v.Verify(t.Context(), bad); !errors.Is(err, domain.ErrUnauthenticated) {
					t.Fatal("malformed/unsupported token accepted")
				}
			}
			attacker := oidcfixture.New(t)
			forged := attacker.Token(t, profile, "subject", map[string]any{"iss": p.Issuer})
			if _, err := v.Verify(t.Context(), forged); !errors.Is(err, domain.ErrUnauthenticated) {
				t.Fatal("untrusted signature accepted")
			}
		})
	}
}

func TestVerifierCachesAndRefreshesSigningKeys(t *testing.T) {
	p := oidcfixture.New(t)
	v, err := New(t.Context(), Config{IssuerURL: p.Issuer, Audience: "froggobank-api", AccessTokenProfile: RFC9068, AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := p.Token(t, RFC9068, "subject", nil)
	for range 2 {
		if _, err := v.Verify(t.Context(), raw); err != nil {
			t.Fatal(err)
		}
	}
	if got := p.JWKSRequests.Load(); got != 1 {
		t.Fatalf("expected cached key, requests=%d", got)
	}
	p.Rotate(t)
	if _, err := v.Verify(t.Context(), p.Token(t, RFC9068, "subject", nil)); err != nil {
		t.Fatalf("rotated key: %v", err)
	}
	if got := p.JWKSRequests.Load(); got != 2 {
		t.Fatalf("expected one rotation refresh, requests=%d", got)
	}
}

func TestRFC9068RequiredClaims(t *testing.T) {
	p := oidcfixture.New(t)
	v, err := New(t.Context(), Config{IssuerURL: p.Issuer, Audience: "froggobank-api", AccessTokenProfile: RFC9068, AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"client_id", "jti", "iat"} {
		if _, err := v.Verify(t.Context(), p.Token(t, RFC9068, "subject", map[string]any{key: nil})); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Fatalf("missing %s accepted: %v", key, err)
		}
	}
}

func TestVerifierConfiguration(t *testing.T) {
	for _, tc := range []Config{
		{IssuerURL: "", Audience: "api", AccessTokenProfile: RFC9068},
		{IssuerURL: "https://issuer.example", AccessTokenProfile: RFC9068},
		{IssuerURL: "https://issuer.example", Audience: "api", AccessTokenProfile: "anything"},
		{IssuerURL: "http://issuer.example", Audience: "api", AccessTokenProfile: RFC9068, AllowInsecureHTTP: true},
		{IssuerURL: "http://127.0.0.1", Audience: "api", AccessTokenProfile: RFC9068},
		{IssuerURL: "https://user:secret@issuer.example", Audience: "api", AccessTokenProfile: RFC9068},
		{IssuerURL: "https://issuer.example?secret=1", Audience: "api", AccessTokenProfile: RFC9068},
	} {
		if err := tc.Validate(); err == nil {
			t.Fatalf("invalid configuration accepted: %+v", tc)
		}
	}
}
