package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/Imanghvs/froggobank/internal/user/domain"
)

const (
	RFC9068       = "rfc9068"
	Keycloak      = "keycloak"
	MaxTokenBytes = 16 * 1024
)

type Config struct {
	IssuerURL          string
	Audience           string
	AccessTokenProfile string
	AllowInsecureHTTP  bool
}

func (c Config) Validate() error {
	if err := validateURL(c.IssuerURL, c.AllowInsecureHTTP); err != nil {
		return fmt.Errorf("invalid OIDC issuer: %w", err)
	}
	if c.Audience == "" || strings.TrimSpace(c.Audience) != c.Audience {
		return errors.New("OIDC audience is required")
	}
	if c.AccessTokenProfile != RFC9068 && c.AccessTokenProfile != Keycloak {
		return errors.New("OIDC access token profile must be rfc9068 or keycloak")
	}
	return nil
}

func validateURL(raw string, allowHTTP bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(raw) != raw {
		return errors.New("expected an absolute provider URL without credentials, query or fragment")
	}
	if u.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(u.Hostname())
	if allowHTTP && u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())) {
		return nil
	}
	return errors.New("HTTPS is required; insecure HTTP is only supported on loopback for local development")
}

type Verifier struct {
	verifier *coreoidc.IDTokenVerifier
	config   Config
}

func New(ctx context.Context, config Config) (*Verifier, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many provider redirects")
		}
		return validateURL(req.URL.String(), config.AllowInsecureHTTP)
	}}
	provider, err := coreoidc.NewProvider(coreoidc.ClientContext(ctx, client), config.IssuerURL)
	if err != nil {
		return nil, errors.New("OIDC provider discovery failed")
	}
	var metadata struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, errors.New("invalid OIDC provider metadata")
	}
	if err := validateURL(metadata.JWKSURI, config.AllowInsecureHTTP); err != nil {
		return nil, fmt.Errorf("invalid OIDC JWKS URL: %w", err)
	}
	return &Verifier{config: config, verifier: provider.Verifier(&coreoidc.Config{
		ClientID:             config.Audience,
		SupportedSigningAlgs: []string{coreoidc.RS256},
	})}, nil
}

func (v *Verifier) Verify(ctx context.Context, raw string) (domain.Identity, error) {
	invalid := domain.ErrUnauthenticated
	if len(raw) > MaxTokenBytes {
		return domain.Identity{}, invalid
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return domain.Identity{}, invalid
	}
	encodedHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return domain.Identity{}, invalid
	}
	var header struct {
		Type string `json:"typ"`
	}
	if err := json.Unmarshal(encodedHeader, &header); err != nil {
		return domain.Identity{}, invalid
	}
	if v.config.AccessTokenProfile == RFC9068 {
		if header.Type != "at+jwt" && header.Type != "application/at+jwt" {
			return domain.Identity{}, invalid
		}
	} else if header.Type != "JWT" {
		return domain.Identity{}, invalid
	}

	// The maintained library verifies signature, algorithm, issuer, audience and
	// expiry and manages cached JWKS refresh. Its ID-token verifier is used only
	// for these primitives: our explicit access-token profile excludes ID tokens.
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return domain.Identity{}, invalid
	}
	var claims struct {
		Type      string `json:"typ"`
		NotBefore *int64 `json:"nbf"`
		ClientID  string `json:"client_id"`
		JWTID     string `json:"jti"`
	}
	if err := token.Claims(&claims); err != nil {
		return domain.Identity{}, invalid
	}
	if v.config.AccessTokenProfile == Keycloak && claims.Type != "Bearer" {
		return domain.Identity{}, invalid
	}
	if v.config.AccessTokenProfile == RFC9068 && (claims.ClientID == "" || claims.JWTID == "" || token.IssuedAt.IsZero()) {
		return domain.Identity{}, invalid
	}
	// Require an exact issuer even for providers for which the library permits
	// legacy aliases. Use strict expiry and not-before checks without its leeway.
	now := time.Now()
	if token.Issuer != v.config.IssuerURL || !token.Expiry.After(now) || (claims.NotBefore != nil && now.Unix() < *claims.NotBefore) {
		return domain.Identity{}, invalid
	}
	identity, err := domain.NewIdentity(token.Issuer, token.Subject)
	if err != nil {
		return domain.Identity{}, invalid
	}
	return identity, nil
}
