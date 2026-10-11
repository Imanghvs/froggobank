package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestErrorResponses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler gin.HandlerFunc
		status  int
		message string
	}{
		{
			name: "bad request",
			handler: func(c *gin.Context) {
				Error(c, http.StatusBadRequest, "invalid request body")
			},
			status: http.StatusBadRequest, message: "invalid request body",
		},
		{name: "unauthorized", handler: Unauthorized, status: http.StatusUnauthorized, message: "unauthenticated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			nextCalled := false
			router.GET("/", tc.handler, func(*gin.Context) { nextCalled = true })
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
			if res.Code != tc.status {
				t.Fatalf("status = %d, want %d", res.Code, tc.status)
			}
			var body map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 1 || body["error"] != tc.message {
				t.Fatalf("unexpected response: %s", res.Body.String())
			}
			if nextCalled {
				t.Fatal("error response did not abort the remaining handlers")
			}
			if tc.status == http.StatusUnauthorized && res.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing bearer challenge")
			}
		})
	}
}
