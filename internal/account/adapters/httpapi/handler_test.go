package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/account/application"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	users "github.com/Imanghvs/froggobank/internal/user/application"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeService struct {
	createAccountFn     func(context.Context, uuid.UUID, string) (domain.Account, error)
	getAccountByIDFn    func(context.Context, uuid.UUID, uuid.UUID) (domain.Account, error)
	getAccountBalanceFn func(context.Context, uuid.UUID, uuid.UUID) (domain.Balance, error)
	getUserAccounts     func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error)
}

func (f fakeService) CreateAccount(ctx context.Context, caller uuid.UUID, currency string) (domain.Account, error) {
	return f.createAccountFn(ctx, caller, currency)
}
func (f fakeService) GetAccountByID(ctx context.Context, caller, id uuid.UUID) (domain.Account, error) {
	return f.getAccountByIDFn(ctx, caller, id)
}
func (f fakeService) GetAccountBalance(ctx context.Context, caller, id uuid.UUID) (domain.Balance, error) {
	return f.getAccountBalanceFn(ctx, caller, id)
}
func (f fakeService) GetUserAccounts(ctx context.Context, caller uuid.UUID, query application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
	return f.getUserAccounts(ctx, caller, query)
}

func setupTestRouter(t *testing.T, s AccountService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := New(s)
	r.POST("/accounts", h.Create)
	r.GET("/accounts/:id", h.GetByID)
	r.GET("/accounts/:id/balance", h.GetBalance)
	r.GET("/accounts", h.GetUserAccounts)
	return r
}
func request(t *testing.T, r *gin.Engine, caller uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(t.Context())
	if caller != uuid.Nil {
		req = req.WithContext(users.WithUser(req.Context(), user.User{ID: caller}))
	}
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	return res
}
func assertError(t *testing.T, res *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if res.Code != status || len(body) != 1 || body["error"] != message {
		t.Fatalf("response %d: %s", res.Code, res.Body.String())
	}
}
func TestCreateUsesAuthenticatedCaller(t *testing.T) {
	caller := uuid.New()
	a, _ := domain.NewCustomer(domain.CurrencyEUR, caller)
	s := fakeService{createAccountFn: func(ctx context.Context, id uuid.UUID, currency string) (domain.Account, error) {
		u, ok := users.UserFromContext(ctx)
		if !ok || u.ID != caller || id != caller || currency != "EUR" {
			t.Error("trusted caller or currency lost")
		}
		return a, nil
	}}
	res := request(t, setupTestRouter(t, s), caller, http.MethodPost, "/accounts", `{"currency":"EUR"}`)
	var body accountResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if res.Code != http.StatusCreated || body.ID != a.ID || body.AccountType != "liability" || !body.EnforceNonnegativeBalance {
		t.Fatalf("create response: %d %s", res.Code, res.Body.String())
	}
}
func TestCreateRejectsClientControlledFieldsAndMalformedJSON(t *testing.T) {
	for _, body := range []string{
		`{"currency":"EUR","owner_id":"` + uuid.NewString() + `"}`,
		`{"currency":"EUR","account_type":"asset"}`,
		`{"currency":"EUR","enforce_nonnegative_balance":false}`,
		`{"currency":123}`, `{"currenc`, `[]`, ``, `{"currency":"EUR"} {"currency":"USD"}`,
	} {
		s := fakeService{createAccountFn: func(context.Context, uuid.UUID, string) (domain.Account, error) {
			t.Fatal("invalid request reached service")
			return domain.Account{}, nil
		}}
		res := request(t, setupTestRouter(t, s), uuid.New(), http.MethodPost, "/accounts", body)
		assertError(t, res, http.StatusBadRequest, "invalid request body")
	}
}
func TestHandlersRequireAuthenticationEvenWithoutMiddleware(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/accounts", `{"currency":"EUR"}`},
		{http.MethodGet, "/accounts/" + uuid.NewString(), ""},
		{http.MethodGet, "/accounts/" + uuid.NewString() + "/balance", ""},
		{http.MethodGet, "/accounts", ""},
		{http.MethodGet, "/accounts?limit=invalid", ""},
	} {
		res := request(t, setupTestRouter(t, fakeService{}), uuid.Nil, tc.method, tc.path, tc.body)
		assertError(t, res, http.StatusUnauthorized, "unauthenticated")
		if res.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Error("missing bearer challenge")
		}
	}
}
func TestCreateErrors(t *testing.T) {
	for _, tc := range []struct {
		err     error
		status  int
		message string
	}{
		{domain.ErrInvalidCurrency, 400, "invalid currency"},
		{user.ErrUnauthenticated, 401, "unauthenticated"},
		{errors.New("secret database detail"), 500, "failed to create account"},
	} {
		s := fakeService{createAccountFn: func(context.Context, uuid.UUID, string) (domain.Account, error) { return domain.Account{}, tc.err }}
		assertError(t, request(t, setupTestRouter(t, s), uuid.New(), http.MethodPost, "/accounts", `{"currency":"EUR"}`), tc.status, tc.message)
	}
}
func TestReadsPassCallerAndPreserveBalancePrecision(t *testing.T) {
	caller := uuid.New()
	a, _ := domain.NewCustomer(domain.CurrencyEUR, caller)
	credits, _ := new(big.Int).SetString("18446744073709551614", 10)
	b, _ := domain.NewBalance(a, big.NewInt(14), credits)
	check := func(id, accID uuid.UUID) {
		if id != caller || accID != a.ID {
			t.Error("request lost caller or account ID")
		}
	}
	s := fakeService{
		getAccountByIDFn:    func(_ context.Context, id, accID uuid.UUID) (domain.Account, error) { check(id, accID); return a, nil },
		getAccountBalanceFn: func(_ context.Context, id, accID uuid.UUID) (domain.Balance, error) { check(id, accID); return b, nil },
	}
	r := setupTestRouter(t, s)
	res := request(t, r, caller, http.MethodGet, "/accounts/"+a.ID.String(), "")
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	res = request(t, r, caller, http.MethodGet, "/accounts/"+a.ID.String()+"/balance", "")
	var body struct {
		Posted  string `json:"posted_minor"`
		Credits string `json:"credits_minor"`
		Scale   int    `json:"scale"`
		Side    string `json:"normal_side"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if res.Code != 200 || body.Posted != "18446744073709551600" || body.Credits != "18446744073709551614" || body.Scale != 2 || body.Side != "credit" {
		t.Fatal(res.Body.String())
	}
}
func TestReadErrors(t *testing.T) {
	for _, balance := range []bool{false, true} {
		for _, tc := range []struct {
			id      string
			err     error
			status  int
			message string
		}{
			{"invalid", nil, 400, "invalid id"},
			{uuid.NewString(), domain.ErrNotFound, 404, "account not found"},
			{uuid.NewString(), user.ErrUnauthenticated, 401, "unauthenticated"},
			{uuid.NewString(), errors.New("secret"), 500, "failed to fetch account"},
		} {
			message := tc.message
			if balance && tc.status == 500 {
				message = "failed to fetch account balance"
			}
			s := fakeService{
				getAccountByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (domain.Account, error) {
					if tc.err == nil {
						t.Fatal("invalid ID reached service")
					}
					return domain.Account{}, tc.err
				},
				getAccountBalanceFn: func(context.Context, uuid.UUID, uuid.UUID) (domain.Balance, error) {
					if tc.err == nil {
						t.Fatal("invalid ID reached service")
					}
					return domain.Balance{}, tc.err
				},
			}
			path := "/accounts/" + tc.id
			if balance {
				path += "/balance"
			}
			assertError(t, request(t, setupTestRouter(t, s), uuid.New(), http.MethodGet, path, ""), tc.status, message)
		}
	}
}

func TestGetUserAccountsUsesCallerAndPagination(t *testing.T) {
	caller := uuid.New()
	acc, err := domain.NewCustomer(domain.CurrencyEUR, caller)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		query    string
		limit    int
		offset   int
		accounts []domain.Account
		currency domain.Currency
	}{
		{"defaults", "", 20, 0, []domain.Account{acc}, ""},
		{"explicit pagination", "?limit=2&offset=3", 2, 3, []domain.Account{acc}, ""},
		{"limit only", "?limit=1", 1, 0, []domain.Account{acc}, ""},
		{"offset only", "?offset=5", 20, 5, []domain.Account{acc}, ""},
		{"maximum limit", "?limit=100&offset=0", 100, 0, []domain.Account{acc}, ""},
		{"empty results", "", 20, 0, []domain.Account{}, ""},
		{"nil results become an empty array", "", 20, 0, nil, ""},
		{"EUR filter with pagination", "?currency=EUR&limit=2&offset=3", 2, 3, []domain.Account{acc}, domain.CurrencyEUR},
		{"USD filter", "?currency=USD", 20, 0, []domain.Account{}, domain.CurrencyUSD},
		{"GBP filter", "?currency=GBP", 20, 0, []domain.Account{}, domain.CurrencyGBP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := fakeService{getUserAccounts: func(ctx context.Context, id uuid.UUID, query application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				calls++
				resolved, ok := users.UserFromContext(ctx)
				if !ok || resolved.ID != caller || id != caller {
					t.Error("authenticated caller or context lost")
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
			res := request(t, setupTestRouter(t, s), caller, http.MethodGet, "/accounts"+tc.query, "")
			if res.Code != http.StatusOK || calls != 1 {
				t.Fatalf("status=%d service calls=%d: %s", res.Code, calls, res.Body.String())
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
			var body paginatedAccountsResponse
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode list response: %v", err)
			}
			if body.Limit != tc.limit || body.Offset != tc.offset || body.Accounts == nil || len(body.Accounts) != len(tc.accounts) {
				t.Fatalf("unexpected list response: %s", res.Body.String())
			}
			for i, want := range tc.accounts {
				got := body.Accounts[i]
				if got.ID != want.ID || got.Currency != string(want.Currency) ||
					got.AccountType != string(want.Type) || got.EnforceNonnegativeBalance != want.EnforceNonnegativeBalance ||
					!got.CreatedAt.Equal(want.CreatedAt) {
					t.Errorf("account %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestGetUserAccountsRejectsInvalidPagination(t *testing.T) {
	for _, query := range []string{
		"?limit=0", "?limit=-1", "?limit=101", "?limit=abc", "?limit=1.5",
		"?limit=999999999999999999999999",
		"?offset=-1", "?offset=abc", "?offset=1.5",
		"?offset=999999999999999999999999",
	} {
		t.Run(query, func(t *testing.T) {
			calls := 0
			s := fakeService{getUserAccounts: func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				calls++
				return domain.PaginatedAccountsResponse{}, nil
			}}
			res := request(t, setupTestRouter(t, s), uuid.New(), http.MethodGet, "/accounts"+query, "")
			if calls != 0 {
				t.Errorf("invalid pagination reached service %d times", calls)
			}
			assertError(t, res, http.StatusBadRequest, "limit must be 1-100 and offset must be nonnegative integers")
		})
	}
}

func TestGetUserAccountsRejectsInvalidCurrency(t *testing.T) {
	for _, query := range []string{
		"?currency=", "?currency", "?currency=JPY", "?currency=eur",
		"?currency=usd", "?currency=gbp", "?currency=%20EUR%20",
	} {
		t.Run(query, func(t *testing.T) {
			calls := 0
			s := fakeService{getUserAccounts: func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				calls++
				return domain.PaginatedAccountsResponse{}, nil
			}}
			res := request(t, setupTestRouter(t, s), uuid.New(), http.MethodGet, "/accounts"+query, "")
			if calls != 0 {
				t.Errorf("invalid currency reached service %d times", calls)
			}
			assertError(t, res, http.StatusBadRequest, "invalid currency")
		})
	}
}

func TestGetUserAccountsErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"unauthenticated", user.ErrUnauthenticated, http.StatusUnauthorized, "unauthenticated"},
		{"service failure", errors.New("secret database detail"), http.StatusInternalServerError, "failed to fetch account"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fakeService{getUserAccounts: func(context.Context, uuid.UUID, application.AccountFilter) (domain.PaginatedAccountsResponse, error) {
				return domain.PaginatedAccountsResponse{}, tc.err
			}}
			res := request(t, setupTestRouter(t, s), uuid.New(), http.MethodGet, "/accounts", "")
			assertError(t, res, tc.status, tc.message)
			if tc.status == http.StatusUnauthorized && res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Error("missing bearer challenge")
			}
		})
	}
}
