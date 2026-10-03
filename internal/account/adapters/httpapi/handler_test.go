package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeService struct {
	createAccountFn  func(context.Context, string) (domain.Account, error)
	getAccountByIDFn func(context.Context, uuid.UUID) (domain.Account, error)
}

func (f fakeService) CreateAccount(ctx context.Context, currencyCode string) (domain.Account, error) {
	return f.createAccountFn(ctx, currencyCode)
}

func (f fakeService) GetAccountByID(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	return f.getAccountByIDFn(ctx, id)
}

func setupTestRouter(t *testing.T, service AccountService) *gin.Engine {
	t.Helper()

	h := New(service)
	router := gin.New()
	router.POST("/accounts", h.Create)
	router.GET("/accounts/:id", h.GetByID)
	return router
}

func postAccount(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(body)).WithContext(t.Context())
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func getAccountByID(t *testing.T, router *gin.Engine, id string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/accounts/"+id, nil).WithContext(t.Context())
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func testAccount() domain.Account {
	return domain.Account{
		ID:        uuid.MustParse("82875e6c-857c-4ca0-9301-af810c68c5a9"),
		Currency:  domain.CurrencyEUR,
		CreatedAt: time.Date(2026, time.January, 2, 3, 4, 5, 123456000, time.UTC),
	}
}

func assertAccountResponse(t *testing.T, res *httptest.ResponseRecorder, status int, acc domain.Account) {
	t.Helper()

	if res.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, res.Code, res.Body.String())
	}
	var body struct {
		ID        uuid.UUID `json:"id"`
		Currency  string    `json:"currency"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.ID != acc.ID || body.Currency != string(acc.Currency) || !body.CreatedAt.Equal(acc.CreatedAt) {
		t.Errorf("expected account %+v, got %+v", acc, body)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &fields); err != nil {
		t.Fatalf("decode response fields: %v", err)
	}
	if len(fields) != 3 {
		t.Errorf("expected only id, currency and created_at fields, got %s", res.Body.String())
	}
}

func assertErrorResponse(t *testing.T, res *httptest.ResponseRecorder, status int, message string) {
	t.Helper()

	if res.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, res.Code, res.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if len(body) != 1 || body["error"] != message {
		t.Errorf("expected error %q, got %s", message, res.Body.String())
	}
}

func TestHandlerCreate(t *testing.T) {
	acc := testAccount()
	calls := 0
	service := fakeService{
		createAccountFn: func(ctx context.Context, currencyCode string) (domain.Account, error) {
			calls++
			if ctx != t.Context() {
				t.Error("expected request context to reach application service")
			}
			if currencyCode != "EUR" {
				t.Errorf("expected currency EUR, got %q", currencyCode)
			}
			return acc, nil
		},
	}

	res := postAccount(t, setupTestRouter(t, service), `{"currency":"EUR"}`)
	assertAccountResponse(t, res, http.StatusCreated, acc)
	if calls != 1 {
		t.Errorf("expected one service call, got %d", calls)
	}
}

func TestHandlerCreateReturnsBadRequestForUnsupportedCurrency(t *testing.T) {
	for _, currency := range []string{"JPY", "", "eur", " EUR "} {
		t.Run(currency, func(t *testing.T) {
			calls := 0
			service := fakeService{
				createAccountFn: func(ctx context.Context, currencyCode string) (domain.Account, error) {
					calls++
					if currencyCode != currency {
						t.Errorf("expected raw currency %q, got %q", currency, currencyCode)
					}
					return domain.Account{}, fmt.Errorf("create account: %w", domain.ErrInvalidCurrency)
				},
			}
			body, err := json.Marshal(map[string]string{"currency": currency})
			if err != nil {
				t.Fatal(err)
			}
			res := postAccount(t, setupTestRouter(t, service), string(body))
			assertErrorResponse(t, res, http.StatusBadRequest, "invalid currency")
			if calls != 1 {
				t.Errorf("expected domain validation through service, got %d calls", calls)
			}
		})
	}
}

func TestHandlerCreateReturnsBadRequestForMalformedJSON(t *testing.T) {
	for _, body := range []string{`{"currenc`, `{"currency":123}`, `[]`, ``} {
		t.Run(body, func(t *testing.T) {
			service := fakeService{
				createAccountFn: func(context.Context, string) (domain.Account, error) {
					t.Fatal("application service must not be called for malformed input")
					return domain.Account{}, nil
				},
			}
			res := postAccount(t, setupTestRouter(t, service), body)
			assertErrorResponse(t, res, http.StatusBadRequest, "invalid request body")
		})
	}
}

func TestHandlerCreateReturnsInternalServerErrorWhenServiceFails(t *testing.T) {
	service := fakeService{
		createAccountFn: func(context.Context, string) (domain.Account, error) {
			return domain.Account{}, errors.New("unexpected persistence error")
		},
	}
	res := postAccount(t, setupTestRouter(t, service), `{"currency":"EUR"}`)
	assertErrorResponse(t, res, http.StatusInternalServerError, "failed to create account")
}

func TestHandlerGetByID(t *testing.T) {
	acc := testAccount()
	calls := 0
	service := fakeService{
		getAccountByIDFn: func(ctx context.Context, id uuid.UUID) (domain.Account, error) {
			calls++
			if ctx != t.Context() {
				t.Error("expected request context to reach application service")
			}
			if id != acc.ID {
				t.Errorf("expected ID %s, got %s", acc.ID, id)
			}
			return acc, nil
		},
	}
	res := getAccountByID(t, setupTestRouter(t, service), acc.ID.String())
	assertAccountResponse(t, res, http.StatusOK, acc)
	if calls != 1 {
		t.Errorf("expected one service call, got %d", calls)
	}
}

func TestHandlerGetByIDReturnsNotFound(t *testing.T) {
	for _, err := range []error{domain.ErrNotFound, fmt.Errorf("lookup: %w", domain.ErrNotFound)} {
		t.Run(err.Error(), func(t *testing.T) {
			service := fakeService{
				getAccountByIDFn: func(context.Context, uuid.UUID) (domain.Account, error) {
					return domain.Account{}, err
				},
			}
			res := getAccountByID(t, setupTestRouter(t, service), uuid.New().String())
			assertErrorResponse(t, res, http.StatusNotFound, "account not found")
		})
	}
}

func TestHandlerGetByIDReturnsBadRequestForInvalidID(t *testing.T) {
	service := fakeService{
		getAccountByIDFn: func(context.Context, uuid.UUID) (domain.Account, error) {
			t.Fatal("application service must not be called for an invalid ID")
			return domain.Account{}, nil
		},
	}
	res := getAccountByID(t, setupTestRouter(t, service), "not-uuid")
	assertErrorResponse(t, res, http.StatusBadRequest, "invalid id")
}

func TestHandlerGetByIDReturnsInternalServerErrorWhenServiceFails(t *testing.T) {
	service := fakeService{
		getAccountByIDFn: func(context.Context, uuid.UUID) (domain.Account, error) {
			return domain.Account{}, errors.New("unexpected persistence error")
		},
	}
	res := getAccountByID(t, setupTestRouter(t, service), uuid.New().String())
	assertErrorResponse(t, res, http.StatusInternalServerError, "failed to fetch account")
}
