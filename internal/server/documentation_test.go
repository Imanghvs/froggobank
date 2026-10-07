package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
)

func TestDocumentationRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Documentation must be available without credentials or a database.
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, httpapi.New(nil), nil, nil)
	for _, tc := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/docs/", "text/html", "./swagger-ui-bundle.js"},
		{"/docs/openapi.yaml", "application/yaml", "openapi: 3.0.3"},
		{"/docs/swagger-ui.css", "text/css", ".swagger-ui"},
		{"/docs/swagger-ui-bundle.js", "javascript", "SwaggerUIBundle"},
		{"/docs/swagger-initializer.js", "javascript", `url: "./openapi.yaml"`},
		{"/docs/LICENSE", "text/plain", "Apache License"},
		{"/docs/NOTICE", "text/plain", "swagger-ui"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if res.Code != http.StatusOK {
				t.Fatalf("got status %d, want 200", res.Code)
			}
			if !strings.Contains(res.Header().Get("Content-Type"), tc.contentType) {
				t.Errorf("unexpected content type: %s", res.Header().Get("Content-Type"))
			}
			if !strings.Contains(res.Body.String(), tc.contains) {
				t.Errorf("response does not contain %q", tc.contains)
			}
			if tc.path == "/docs/openapi.yaml" && !strings.Contains(res.Body.String(), "- url: /\n") {
				t.Error("API server URL must be relative to the documentation origin")
			}
		})
	}

	t.Run("redirect to trailing slash", func(t *testing.T) {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/docs", nil))
		if res.Code != http.StatusTemporaryRedirect || res.Header().Get("Location") != "/docs/" {
			t.Fatalf("unexpected redirect: status=%d location=%q", res.Code, res.Header().Get("Location"))
		}
	})

	t.Run("unlisted documentation files are not exposed", func(t *testing.T) {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/docs/assets.go", nil))
		if res.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want 404", res.Code)
		}
	})
}
