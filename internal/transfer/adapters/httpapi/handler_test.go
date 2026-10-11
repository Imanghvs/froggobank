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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Imanghvs/froggobank/internal/transfer/domain"
	users "github.com/Imanghvs/froggobank/internal/user/application"
	user "github.com/Imanghvs/froggobank/internal/user/domain"
)

type fakeService struct {
	transferFn func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (domain.Transfer, error)
}

func (f fakeService) Transfer(ctx context.Context, caller, source, destination uuid.UUID, amount string) (domain.Transfer, error) {
	return f.transferFn(ctx, caller, source, destination, amount)
}

func TestTransferErrorResponses(t *testing.T) {
	caller, source, destination := uuid.New(), uuid.New(), uuid.New()
	body := fmt.Sprintf(`{"source":%q,"destination":%q,"amount_minor":"2500"}`, source, destination)
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"unauthenticated", user.ErrUnauthenticated, http.StatusUnauthorized, "unauthenticated"},
		{"invalid currency", domain.ErrInvalidCurrency, http.StatusBadRequest, "invalid currency"},
		{"invalid amount", domain.ErrInvalidAmount, http.StatusBadRequest, "invalid amount"},
		{"invalid account IDs", domain.ErrInvalidAccountID, http.StatusBadRequest, "invalid source or destination account ID"},
		{"identical accounts", domain.ErrSameAccount, http.StatusBadRequest, "source and destination accounts must be different"},
		{"source not found", domain.ErrSourceNotFound, http.StatusNotFound, "source account not found"},
		{"destination not found", domain.ErrDestinationNotFound, http.StatusNotFound, "destination account not found"},
		{"currency mismatch", domain.ErrCurrencyMismatch, http.StatusBadRequest, "account currencies must match the transfer currency"},
		{"unexpected failure", errors.New("private database connection details"), http.StatusInternalServerError, "failed to create transfer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := New(fakeService{transferFn: func(ctx context.Context, id, src, dst uuid.UUID, amount string) (domain.Transfer, error) {
				calls++
				resolved, ok := users.UserFromContext(ctx)
				if !ok || resolved.ID != caller || id != caller || src != source || dst != destination || amount != "2500" {
					t.Fatal("transfer request or authenticated caller was not forwarded")
				}
				return domain.Transfer{}, fmt.Errorf("transfer: %w", tc.err)
			}})
			router := gin.New()
			router.POST("/transfers", h.Transfer)
			req := httptest.NewRequest(http.MethodPost, "/transfers", strings.NewReader(body)).WithContext(users.WithUser(t.Context(), user.User{ID: caller}))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			assertTransferError(t, res, tc.status, tc.message)
			if calls != 1 {
				t.Fatalf("service calls = %d, want 1", calls)
			}
			if tc.status == http.StatusUnauthorized && res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing bearer challenge")
			}
		})
	}
}

func TestTransferRejectsRequestsBeforeServiceCall(t *testing.T) {
	for _, tc := range []struct {
		name          string
		body          string
		authenticated bool
		status        int
		message       string
	}{
		{"missing caller", `{}`, false, http.StatusUnauthorized, "unauthenticated"},
		{"malformed JSON", `{"source":`, true, http.StatusBadRequest, "invalid request body"},
		{"invalid UUID", `{"source":"invalid"}`, true, http.StatusBadRequest, "invalid request body"},
		{"unknown field", `{"owner_id":"untrusted"}`, true, http.StatusBadRequest, "invalid request body"},
		{"request currency", `{"currency":"EUR"}`, true, http.StatusBadRequest, "invalid request body"},
		{"extra JSON", `{} {}`, true, http.StatusBadRequest, "invalid request body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := New(fakeService{transferFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (domain.Transfer, error) {
				t.Fatal("rejected request reached service")
				return domain.Transfer{}, nil
			}})
			router := gin.New()
			router.POST("/transfers", h.Transfer)
			req := httptest.NewRequest(http.MethodPost, "/transfers", strings.NewReader(tc.body)).WithContext(t.Context())
			if tc.authenticated {
				req = req.WithContext(users.WithUser(req.Context(), user.User{ID: uuid.New()}))
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			assertTransferError(t, res, tc.status, tc.message)
		})
	}
}

func assertTransferError(t *testing.T, res *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected a single JSON error response: %v; body=%s", err, res.Body.String())
	}
	if res.Code != status || len(body) != 1 || body["error"] != message {
		t.Fatalf("response = %d %s, want %d %q", res.Code, res.Body.String(), status, message)
	}
}
