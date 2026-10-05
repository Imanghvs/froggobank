// Package oidcfixture supplies a local discovery/JWKS server and real signed
// JWTs for automated tests. It never contacts a live identity provider.
package oidcfixture

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
)

type Provider struct {
	Issuer       string
	server       *httptest.Server
	mu           sync.RWMutex
	key          *rsa.PrivateKey
	kid          string
	JWKSRequests atomic.Int64
}

func New(t *testing.T) *Provider {
	t.Helper()
	p := &Provider{}
	p.Rotate(t)
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": p.Issuer, "jwks_uri": p.Issuer + "/jwks",
				"authorization_endpoint": p.Issuer + "/authorize", "token_endpoint": p.Issuer + "/token",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/jwks":
			p.JWKSRequests.Add(1)
			p.mu.RLock()
			key := jose.JSONWebKey{Key: &p.key.PublicKey, KeyID: p.kid, Algorithm: "RS256", Use: "sig"}
			p.mu.RUnlock()
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
		default:
			http.NotFound(w, r)
		}
	}))
	p.Issuer = p.server.URL
	t.Cleanup(p.server.Close)
	return p
}

func (p *Provider) Rotate(t *testing.T) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.key = key
	p.kid = uuid.NewString()
}

func (p *Provider) Token(t *testing.T, profile, subject string, overrides map[string]any) string {
	t.Helper()
	typ := "at+jwt"
	claims := map[string]any{
		"iss": p.Issuer, "sub": subject, "aud": []string{"froggobank-api"},
		"iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
		"jti": uuid.NewString(), "client_id": "froggobank-web",
	}
	if profile == "keycloak" {
		typ = "JWT"
		claims["typ"] = "Bearer"
	}
	for key, value := range overrides {
		if value == nil {
			delete(claims, key)
		} else {
			claims[key] = value
		}
	}
	return p.Sign(t, typ, claims)
}

func (p *Provider) Sign(t *testing.T, typ string, claims map[string]any) string {
	t.Helper()
	p.mu.RLock()
	key, kid := p.key, p.kid
	p.mu.RUnlock()
	options := new(jose.SignerOptions).WithType(jose.ContentType(typ)).WithHeader("kid", kid)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, options)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
