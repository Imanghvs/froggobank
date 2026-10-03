package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
	"github.com/Imanghvs/froggobank/internal/account/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	currencyCode string,
) (domain.Account, error) {
	return f.account, nil
}

func (f fakeAccountService) GetAccountByID(
	ctx context.Context,
	id uuid.UUID,
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
	router := NewRouter(logger, fakeDatabase{}, handler)

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

	router := NewRouter(logger, fakeDatabase{}, httpapi.New(fakeAccountService{}))

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

func TestReadyWhenDatabaseIsAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)

	database := fakeDatabase{}

	router := NewRouter(logger, database, httpapi.New(fakeAccountService{}))

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

	router := NewRouter(logger, database, httpapi.New(fakeAccountService{}))

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

	router := NewRouter(logger, database, httpapi.New(fakeAccountService{}))

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
