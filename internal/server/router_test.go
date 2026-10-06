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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	users "github.com/Imanghvs/froggobank/internal/user/application"
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
	account           domain.Account
	getUserAccountsFn func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error)
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

func (f fakeAccountService) GetUserAccounts(
	ctx context.Context,
	callerID uuid.UUID,
	query application.AccountFilter,
) (domain.PaginatedAccountsResponse, error) {
	if f.getUserAccountsFn != nil {
		return f.getUserAccountsFn(ctx, callerID, query)
	}
	var resAccounts []domain.Account
	return domain.PaginatedAccountsResponse{
		Accounts: resAccounts,
		Limit:    query.Limit,
		Offset:   query.Offset,
	}, nil
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
	for _, path := range []string{"/me", "/accounts", "/accounts/" + uuid.NewString(), "/accounts/" + uuid.NewString() + "/balance"} {
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

func TestListAccountsRouteUsesAuthenticatedCallerAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	caller := uuid.MustParse("246d4d77-07c1-40dc-9b28-9af57a2d4cb9")
	acc, err := domain.NewCustomer(domain.CurrencyEUR, caller)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		path     string
		limit    int
		offset   int
		accounts []domain.Account
		currency domain.Currency
	}{
		{"defaults", "/accounts", 20, 0, []domain.Account{acc}, ""},
		{"explicit pagination", "/accounts?limit=2&offset=3", 2, 3, []domain.Account{acc}, ""},
		{"minimum limit", "/accounts?limit=1", 1, 0, []domain.Account{acc}, ""},
		{"maximum limit", "/accounts?limit=100", 100, 0, []domain.Account{acc}, ""},
		{"offset only", "/accounts?offset=4", 20, 4, []domain.Account{acc}, ""},
		{"empty page", "/accounts?limit=2&offset=100", 2, 100, []domain.Account{}, ""},
		{"nil results become an empty array", "/accounts", 20, 0, nil, ""},
		{"EUR filter with pagination", "/accounts?currency=EUR&limit=2&offset=3", 2, 3, []domain.Account{acc}, domain.CurrencyEUR},
		{"USD filter", "/accounts?currency=USD", 20, 0, []domain.Account{}, domain.CurrencyUSD},
		{"GBP filter", "/accounts?currency=GBP", 20, 0, []domain.Account{}, domain.CurrencyGBP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			service := fakeAccountService{getUserAccountsFn: func(ctx context.Context, id uuid.UUID, query application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				calls++
				resolved, ok := users.UserFromContext(ctx)
				if !ok || resolved.ID != caller || id != caller {
					t.Error("authenticated caller did not reach the supplied service")
				}
				if query.Limit != tc.limit || query.Offset != tc.offset {
					t.Errorf("got limit=%d offset=%d, want limit=%d offset=%d", query.Limit, query.Offset, tc.limit, tc.offset)
				}
				if tc.currency == "" {
					if query.Currency != nil {
						t.Errorf("omitted currency should produce a nil filter, got %q", *query.Currency)
					}
				} else if query.Currency == nil || *query.Currency != tc.currency {
					t.Errorf("currency filter was not forwarded: got %v, want %q", query.Currency, tc.currency)
				}
				return domain.PaginatedAccountsResponse{Accounts: tc.accounts, Limit: query.Limit, Offset: query.Offset}, nil
			}}
			router := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), fakeDatabase{}, httpapi.New(service), testVerifier{}, testUsers{})
			req := httptest.NewRequest(http.MethodGet, tc.path, nil).WithContext(t.Context())
			req.Header.Set("Authorization", "Bearer test-token")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != http.StatusOK || calls != 1 {
				t.Fatalf("response status=%d service calls=%d: %s", res.Code, calls, res.Body.String())
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(res.Body.Bytes(), &raw); err != nil {
				t.Fatalf("decode response keys: %v", err)
			}
			if len(raw) != 3 {
				t.Fatalf("expected only accounts, limit, and offset: %s", res.Body.String())
			}
			for _, key := range []string{"accounts", "limit", "offset"} {
				if _, ok := raw[key]; !ok {
					t.Fatalf("missing JSON key %q: %s", key, res.Body.String())
				}
			}
			var rawAccounts []map[string]json.RawMessage
			if err := json.Unmarshal(raw["accounts"], &rawAccounts); err != nil {
				t.Fatalf("decode account keys: %v", err)
			}
			if rawAccounts == nil {
				t.Fatal("accounts must be an array, not null")
			}
			for _, rawAccount := range rawAccounts {
				if len(rawAccount) != 5 {
					t.Fatalf("unexpected account fields (owner ID must be excluded): %s", res.Body.String())
				}
				for _, key := range []string{"id", "currency", "account_type", "enforce_nonnegative_balance", "created_at"} {
					if _, ok := rawAccount[key]; !ok {
						t.Fatalf("missing account JSON key %q", key)
					}
				}
			}
			var body struct {
				Accounts []struct {
					ID                        uuid.UUID `json:"id"`
					Currency                  string    `json:"currency"`
					AccountType               string    `json:"account_type"`
					EnforceNonnegativeBalance bool      `json:"enforce_nonnegative_balance"`
					CreatedAt                 time.Time `json:"created_at"`
				} `json:"accounts"`
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode list response: %v", err)
			}
			if body.Limit != tc.limit || body.Offset != tc.offset || body.Accounts == nil || len(body.Accounts) != len(tc.accounts) {
				t.Fatalf("unexpected list response: %s", res.Body.String())
			}
			for i, want := range tc.accounts {
				got := body.Accounts[i]
				if got.ID != want.ID || got.Currency != string(want.Currency) || got.AccountType != string(want.Type) ||
					got.EnforceNonnegativeBalance != want.EnforceNonnegativeBalance || !got.CreatedAt.Equal(want.CreatedAt) {
					t.Errorf("account %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestListAccountsRouteRejectsRequestsBeforeServiceCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		query  string
		header string
		status int
	}{
		{"missing token", "", "", http.StatusUnauthorized},
		{"invalid token", "", "Bearer invalid", http.StatusUnauthorized},
		{"wrong auth scheme", "", "Basic abc", http.StatusUnauthorized},
		{"authentication precedes pagination", "?limit=invalid", "", http.StatusUnauthorized},
		{"authentication precedes currency validation", "?currency=JPY", "", http.StatusUnauthorized},
		{"empty currency", "?currency=", "Bearer test-token", http.StatusBadRequest},
		{"bare currency parameter", "?currency", "Bearer test-token", http.StatusBadRequest},
		{"unsupported currency", "?currency=JPY", "Bearer test-token", http.StatusBadRequest},
		{"lowercase EUR", "?currency=eur", "Bearer test-token", http.StatusBadRequest},
		{"lowercase USD", "?currency=usd", "Bearer test-token", http.StatusBadRequest},
		{"lowercase GBP", "?currency=gbp", "Bearer test-token", http.StatusBadRequest},
		{"padded currency", "?currency=%20EUR%20", "Bearer test-token", http.StatusBadRequest},
		{"zero limit", "?limit=0", "Bearer test-token", http.StatusBadRequest},
		{"negative limit", "?limit=-1", "Bearer test-token", http.StatusBadRequest},
		{"excessive limit", "?limit=101", "Bearer test-token", http.StatusBadRequest},
		{"nonnumeric limit", "?limit=abc", "Bearer test-token", http.StatusBadRequest},
		{"overflow limit", "?limit=999999999999999999999999", "Bearer test-token", http.StatusBadRequest},
		{"negative offset", "?offset=-1", "Bearer test-token", http.StatusBadRequest},
		{"nonnumeric offset", "?offset=abc", "Bearer test-token", http.StatusBadRequest},
		{"fractional offset", "?offset=1.5", "Bearer test-token", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			service := fakeAccountService{getUserAccountsFn: func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				calls++
				return domain.PaginatedAccountsResponse{}, nil
			}}
			router := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), fakeDatabase{}, httpapi.New(service), testVerifier{}, testUsers{})
			req := httptest.NewRequest(http.MethodGet, "/accounts"+tc.query, nil)
			req.Header.Set("Authorization", tc.header)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.status || calls != 0 {
				t.Errorf("got status=%d service calls=%d, want status=%d calls=0", res.Code, calls, tc.status)
			}
			var body map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("expected a single JSON error response: %v; body=%s", err, res.Body.String())
			}
			if len(body) != 1 || body["error"] == "" {
				t.Errorf("unexpected error response: %s", res.Body.String())
			}
			if tc.status == http.StatusBadRequest && strings.HasPrefix(tc.query, "?currency") && body["error"] != "invalid currency" {
				t.Errorf("got error %q, want invalid currency", body["error"])
			}
			if tc.status == http.StatusUnauthorized && res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Error("missing Bearer challenge")
			}
		})
	}
}

func TestListAccountsRouteHandlesServiceErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"unauthenticated", user.ErrUnauthenticated, http.StatusUnauthorized},
		{"service failure", errors.New("private database connection details"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := fakeAccountService{getUserAccountsFn: func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				return domain.PaginatedAccountsResponse{}, tc.err
			}}
			router := NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), fakeDatabase{}, httpapi.New(service), testVerifier{}, testUsers{})
			req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
			req.Header.Set("Authorization", "Bearer test-token")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("got status %d, want %d: %s", res.Code, tc.status, res.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 1 || body["error"] == "" || strings.Contains(res.Body.String(), "private database") {
				t.Errorf("unexpected error response: %s", res.Body.String())
			}
		})
	}
}
