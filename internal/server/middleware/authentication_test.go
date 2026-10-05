package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Imanghvs/froggobank/internal/user/application"
	"github.com/Imanghvs/froggobank/internal/user/domain"
	"github.com/gin-gonic/gin"
)

type fakeVerifier struct {
	verify func(context.Context, string) (domain.Identity, error)
}

func (f fakeVerifier) Verify(ctx context.Context, raw string) (domain.Identity, error) {
	return f.verify(ctx, raw)
}

type fakeUserResolver struct {
	resolve func(context.Context, domain.Identity) (domain.User, error)
}

func (f fakeUserResolver) ResolveIdentity(ctx context.Context, i domain.Identity) (domain.User, error) {
	return f.resolve(ctx, i)
}

func TestAuthenticationRejectsMalformedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, headers := range [][]string{nil, {""}, {"Basic secret"}, {"Bearer"}, {"Bearer token extra"}, {"Bearer one", "Bearer two"}, {"Bearer " + strings.Repeat("x", 16*1024+1)}} {
		r := gin.New()
		r.Use(Authentication(fakeVerifier{verify: func(context.Context, string) (domain.Identity, error) {
			t.Fatal("malformed header reached verifier")
			return domain.Identity{}, nil
		}}, fakeUserResolver{}))
		r.GET("/protected", func(c *gin.Context) { t.Fatal("unauthenticated handler executed") })
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		for _, value := range headers {
			req.Header.Add("Authorization", value)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != 401 || res.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("header %v: %d", headers, res.Code)
		}
	}
}
func TestAuthenticationAndLogs(t *testing.T) {
	identity, _ := domain.NewIdentity("https://issuer.example", "subject")
	user, _ := domain.New(identity)
	for _, tc := range []struct {
		name                  string
		verifyErr, resolveErr error
		status                int
	}{
		{"valid", nil, nil, 200},
		{"bad token", errors.New("secret-token"), nil, 401},
		{"user database failure", nil, errors.New("secret-token"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			r := gin.New()
			r.Use(RequestLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
			resolved := false
			r.Use(Authentication(fakeVerifier{verify: func(ctx context.Context, raw string) (domain.Identity, error) {
				if ctx != t.Context() || raw != "secret-token" {
					t.Error("verification context/token lost")
				}
				return identity, tc.verifyErr
			}}, fakeUserResolver{resolve: func(ctx context.Context, got domain.Identity) (domain.User, error) {
				resolved = true
				if got != identity || ctx != t.Context() {
					t.Error("resolved unverified identity")
				}
				return user, tc.resolveErr
			}}))
			r.GET("/protected", func(c *gin.Context) {
				got, ok := application.UserFromContext(c.Request.Context())
				if !ok || got != user {
					t.Error("trusted user context missing")
				}
				c.Status(200)
			})
			req := httptest.NewRequest(http.MethodGet, "/protected?token=secret-token", nil).WithContext(t.Context())
			req.Header.Set("Authorization", "bEaReR secret-token")
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("response %d %s", res.Code, res.Body.String())
			}
			if tc.verifyErr != nil && resolved {
				t.Error("invalid token provisioned a user")
			}
			if strings.Contains(logs.String(), "secret-token") || strings.Contains(res.Body.String(), "secret-token") {
				t.Fatal("credential leaked")
			}
		})
	}
}
