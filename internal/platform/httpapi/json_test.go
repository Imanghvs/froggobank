package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStrictJSON(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		wantErr  bool
		tooLarge bool
	}{
		{name: "valid", body: `{"currency":"EUR"}`},
		{name: "trailing whitespace", body: "{\"currency\":\"EUR\"}\n\t "},
		{name: "unknown field", body: `{"currency":"EUR","owner_id":"untrusted"}`, wantErr: true},
		{name: "second JSON value", body: `{"currency":"EUR"} {}`, wantErr: true},
		{name: "trailing garbage", body: `{"currency":"EUR"} invalid`, wantErr: true},
		{name: "empty body", body: "", wantErr: true},
		{name: "malformed JSON", body: `{"currency":`, wantErr: true},
		{name: "wrong field type", body: `{"currency":123}`, wantErr: true},
		{name: "binding validation", body: `{}`, wantErr: true},
		{name: "oversized value", body: `{"currency":"` + strings.Repeat("A", maxJSONBodyBytes) + `"}`, wantErr: true, tooLarge: true},
		{name: "oversized trailing whitespace", body: `{"currency":"EUR"}` + strings.Repeat(" ", maxJSONBodyBytes), wantErr: true, tooLarge: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			var dst struct {
				Currency string `json:"currency" binding:"required"`
			}
			err := c.ShouldBindWith(&dst, StrictJSON{})
			if (err != nil) != tc.wantErr {
				t.Fatalf("binding error = %v, want error = %v", err, tc.wantErr)
			}
			if !tc.wantErr && dst.Currency != "EUR" {
				t.Fatalf("currency = %q, want EUR", dst.Currency)
			}
			if tc.tooLarge {
				var sizeErr *http.MaxBytesError
				if !errors.As(err, &sizeErr) {
					t.Fatalf("expected body size error, got %v", err)
				}
			}
		})
	}
}
