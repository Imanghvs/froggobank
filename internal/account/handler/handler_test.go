package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Imanghvs/froggobank/internal/account"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeRepository struct {
	createFn  func(context.Context, account.Account) error
	getByIDFn func(context.Context, uuid.UUID) (account.Account, error)
}

func (f *fakeRepository) Create(ctx context.Context, acc account.Account) error {
	if f.createFn != nil {
		return f.createFn(ctx, acc)
	}
	return nil
}

func (f *fakeRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (account.Account, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return account.Account{}, nil
}

func setupTestRouter(t *testing.T, repo account.Repository) *gin.Engine {
	t.Helper()

	h := New(repo)

	router := gin.New()
	router.POST("/accounts", h.Create)
	router.GET("/accounts/:id", h.GetByID)

	return router
}

func postAccount(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodPost,
		"/accounts",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	return res
}

func getAccountByID(t *testing.T, router *gin.Engine, id string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodGet,
		"/accounts/"+id,
		nil,
	)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	return res
}

func TestHandlerCreate(t *testing.T) {
	var createdAccount account.Account

	repo := &fakeRepository{
		createFn: func(ctx context.Context, acc account.Account) error {
			createdAccount = acc
			return nil
		},
	}

	router := setupTestRouter(t, repo)

	res := postAccount(t, router, `{"currency":"EUR"}`)

	if res.Code != http.StatusCreated {
		t.Errorf("expected status %d, received %d", http.StatusCreated, res.Code)
	}

	if createdAccount.Currency != account.CurrencyEUR {
		t.Errorf(
			"expected currency %q, received %q",
			account.CurrencyEUR,
			createdAccount.Currency,
		)
	}

	if createdAccount.ID == uuid.Nil {
		t.Error("expected account ID to be generated")
	}

	var body accountResponse

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if body.ID != createdAccount.ID {
		t.Errorf(
			"expected response id %q, received %q",
			createdAccount.ID,
			body.ID,
		)
	}

	if body.Currency != createdAccount.Currency {
		t.Errorf(
			"expected response currency %q, received %q",
			createdAccount.Currency,
			body.Currency,
		)
	}

	if !body.CreatedAt.Equal(createdAccount.CreatedAt) {
		t.Errorf(
			"expected response created_at %v, received %v",
			createdAccount.CreatedAt,
			body.CreatedAt,
		)
	}
}

func TestHandlerCreateReturnsBadRequestForUnsupportedCurrency(t *testing.T) {
	createCalled := false

	repo := &fakeRepository{
		createFn: func(ctx context.Context, acc account.Account) error {
			createCalled = true
			return nil
		},
	}

	router := setupTestRouter(t, repo)

	res := postAccount(t, router, `{"currency":"JPY"}`)

	if res.Code != http.StatusBadRequest {
		t.Errorf(
			"expected status %d, received %d",
			http.StatusBadRequest,
			res.Code,
		)
	}

	if createCalled {
		t.Error("expected repository Create not to be called")
	}
}

func TestHandlerCreateReturnsBadRequestForMalformedJSON(t *testing.T) {
	createCalled := false

	repo := &fakeRepository{
		createFn: func(ctx context.Context, acc account.Account) error {
			createCalled = true
			return nil
		},
	}

	router := setupTestRouter(t, repo)

	res := postAccount(t, router, `{"currenc`)

	if res.Code != http.StatusBadRequest {
		t.Errorf(
			"expected status %d, received %d",
			http.StatusBadRequest,
			res.Code,
		)
	}

	if createCalled {
		t.Error("expected repository Create not to be called")
	}
}

func TestHandlerCreateReturnsInternalServerErrorWhenRepositoryFails(t *testing.T) {
	repo := &fakeRepository{
		createFn: func(ctx context.Context, acc account.Account) error {
			return errors.New("unexpected db error")
		},
	}

	router := setupTestRouter(t, repo)

	res := postAccount(t, router, `{"currency":"EUR"}`)

	if res.Code != http.StatusInternalServerError {
		t.Errorf(
			"expected status code %d, received %d",
			http.StatusInternalServerError,
			res.Code,
		)
	}
}

func TestHandlerGetByID(t *testing.T) {
	acc := account.New(account.CurrencyEUR)
	repo := &fakeRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (account.Account, error) {
			if id != acc.ID {
				t.Errorf("expected id %q, received %q", acc.ID, id)
			}
			return acc, nil
		},
	}

	router := setupTestRouter(t, repo)

	res := getAccountByID(t, router, acc.ID.String())

	if res.Code != http.StatusOK {
		t.Errorf(
			"expected status code %d, received %d",
			http.StatusOK,
			res.Code,
		)
	}

	var body accountResponse

	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if body.ID != acc.ID {
		t.Errorf("expected id %q, received %q", acc.ID, body.ID)
	}

	if body.Currency != acc.Currency {
		t.Errorf(
			"expected currency %q, received %q",
			acc.Currency,
			body.Currency,
		)
	}

	if !body.CreatedAt.Equal(acc.CreatedAt) {
		t.Errorf(
			"expected created_at %v, received %v",
			acc.CreatedAt,
			body.CreatedAt,
		)
	}
}

func TestHandlerGetByIDReturnsNotFound(t *testing.T) {
	repo := &fakeRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (account.Account, error) {
			return account.Account{}, account.ErrNotFound
		},
	}

	id := uuid.New()

	router := setupTestRouter(t, repo)
	res := getAccountByID(t, router, id.String())

	if res.Code != http.StatusNotFound {
		t.Errorf(
			"expected status code %d, received %d",
			http.StatusNotFound,
			res.Code,
		)
	}
}

func TestHandlerGetByIDReturnsBadRequestForInvalidID(t *testing.T) {
	getByIDCalled := false
	repo := &fakeRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (account.Account, error) {
			getByIDCalled = true
			return account.Account{}, nil
		},
	}

	router := setupTestRouter(t, repo)
	res := getAccountByID(t, router, "not-uuid")

	if res.Code != http.StatusBadRequest {
		t.Errorf(
			"expected status code %d, received %d",
			http.StatusBadRequest,
			res.Code,
		)
	}

	if getByIDCalled {
		t.Errorf("expected GetByID not to be called")
	}
}

func TestHandlerGetByIDReturnsInternalServerErrorWhenRepositoryFails(t *testing.T) {
	repo := &fakeRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (account.Account, error) {
			return account.Account{}, errors.New("unexpected database error")
		},
	}

	router := setupTestRouter(t, repo)

	res := getAccountByID(t, router, uuid.New().String())

	if res.Code != http.StatusInternalServerError {
		t.Errorf(
			"expected status code %d, received %d",
			http.StatusInternalServerError,
			res.Code,
		)
	}
}
