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

	"github.com/Imanghvs/froggobank/internal/account/domain"
	users "github.com/Imanghvs/froggobank/internal/user/application"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeService struct {
	createAccountFn     func(context.Context, uuid.UUID, string) (domain.Account, error)
	getAccountByIDFn    func(context.Context, uuid.UUID, uuid.UUID) (domain.Account, error)
	getAccountBalanceFn func(context.Context, uuid.UUID, uuid.UUID) (domain.Balance, error)
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
func setupTestRouter(t *testing.T, s AccountService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := New(s)
	r.POST("/accounts", h.Create)
	r.GET("/accounts/:id", h.GetByID)
	r.GET("/accounts/:id/balance", h.GetBalance)
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
