package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeDatabase struct {
	err    error
	pingFn func(context.Context) error
}

func (f fakeDatabase) Ping(ctx context.Context) error {
	if f.pingFn != nil {
		return f.pingFn(ctx)
	}
	return f.err
}

type fakeAccountService struct {
	account domain.Account
}

func (f fakeAccountService) CreateAccount(
	ctx context.Context,
	callerID uuid.UUID,
	currencyCode string,
) (domain.Account, error) {
	return f.account, nil
}

func (f fakeAccountService) GetAccountBalance(ctx context.Context, callerID, id uuid.UUID) (domain.Balance, error) {
	if id != f.account.ID {
		return domain.Balance{}, domain.ErrNotFound
	}
	return domain.NewBalance(f.account, big.NewInt(0), big.NewInt(0))
}

func (f fakeAccountService) GetAccountByID(
	ctx context.Context,
	callerID, id uuid.UUID,
) (domain.Account, error) {
	if id == f.account.ID {
		return f.account, nil
	}
	return domain.Account{}, domain.ErrNotFound
}

func TestAccountRoutesUseSuppliedHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	acc := domain.New(domain.CurrencyEUR)
	handler := httpapi.New(fakeAccountService{account: acc})
	router := NewRouter(logger, fakeDatabase{}, handler, testVerifier{}, testUsers{})

	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPost, "/accounts", `{"currency":"EUR"}`, http.StatusCreated},
		{http.MethodGet, "/accounts/" + acc.ID.String(), "", http.StatusOK},
	} {
		t.Run(tc.method, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer test-token")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("expected status %d, got %d: %s", tc.status, recorder.Code, recorder.Body.String())
			}
			var response struct {
				ID uuid.UUID `json:"id"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.ID != acc.ID {
				t.Errorf("expected supplied handler's account ID %s, got %s", acc.ID, response.ID)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)

	router := NewRouter(logger, fakeDatabase{}, httpapi.New(fakeAccountService{}), testVerifier{}, testUsers{})

	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "ok" {
		t.Fatalf("expected status %q got %q", "ok", response.Status)
	}
}

func TestAccountBalanceRouteUsesSuppliedHandler(t *testing.T) {
	acc := domain.New(domain.CurrencyEUR)
	router := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), fakeDatabase{}, httpapi.New(fakeAccountService{account: acc}), testVerifier{}, testUsers{})
	res := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/accounts/"+acc.ID.String()+"/balance", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	router.ServeHTTP(res, request)
	var body struct {
		AccountID uuid.UUID `json:"account_id"`
		Posted    string    `json:"posted_minor"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if res.Code != http.StatusOK || body.AccountID != acc.ID || body.Posted != "0" {
		t.Fatalf("balance route: %d %s", res.Code, res.Body.String())
	}
}

func TestReadyWhenDatabaseIsAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)

	database := fakeDatabase{}

	router := NewRouter(logger, database, httpapi.New(fakeAccountService{}), testVerifier{}, testUsers{})

	request := httptest.NewRequest(
		http.MethodGet,
		"/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "ready" {
		t.Fatalf(
			"expected status %q, got %q",
			"ready",
			response.Status,
		)
	}
}

func TestNotReadyWhenDatabaseIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)

	database := fakeDatabase{
		err: errors.New("database unavailable"),
	}

	router := NewRouter(logger, database, httpapi.New(fakeAccountService{}), testVerifier{}, testUsers{})

	request := httptest.NewRequest(
		http.MethodGet,
		"/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			recorder.Code,
		)
	}

	var response struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "not_ready" {
		t.Fatalf(
			"expected status %q, got %q",
			"not_ready",
			response.Status,
		)
	}
}

func TestReadyUsesDatabaseTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)

	hasDeadline := false

	database := fakeDatabase{
		pingFn: func(ctx context.Context) error {
			_, hasDeadline = ctx.Deadline()
			return nil
		},
	}

	router := NewRouter(logger, database, httpapi.New(fakeAccountService{}), testVerifier{}, testUsers{})

	request := httptest.NewRequest(
		http.MethodGet,
		"/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if !hasDeadline {
		t.Fatal("expected database ping context to have a deadline")
	}
}

type testVerifier struct{}

func (testVerifier) Verify(ctx context.Context, token string) (user.Identity, error) {
	if token != "test-token" {
		return user.Identity{}, user.ErrUnauthenticated
	}
	return user.NewIdentity("https://issuer.example", "subject")
}

type testUsers struct{}

func (testUsers) ResolveIdentity(ctx context.Context, identity user.Identity) (user.User, error) {
	return user.User{ID: uuid.MustParse("246d4d77-07c1-40dc-9b28-9af57a2d4cb9"), Identity: identity}, nil
}
func TestProtectedRoutesRejectMissingOrInvalidTokens(t *testing.T) {
	router := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), fakeDatabase{}, httpapi.New(fakeAccountService{}), testVerifier{}, testUsers{})
	for _, path := range []string{"/me", "/accounts/" + uuid.NewString(), "/accounts/" + uuid.NewString() + "/balance"} {
		for _, header := range []string{"", "Bearer invalid", "Basic abc"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
			}
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`{"currency":"EUR"}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatal(res.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "246d4d77-07c1-40dc-9b28-9af57a2d4cb9") {
		t.Fatal(res.Body.String())
	}
}
