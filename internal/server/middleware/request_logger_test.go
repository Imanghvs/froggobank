package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var output bytes.Buffer

	logger := slog.New(
		slog.NewJSONHandler(&output, nil),
	)

	router := gin.New()

	router.Use(RequestLogger(logger))

	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(
		http.MethodGet,
		"/health?token=secret",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	var entry map[string]any

	if err := json.Unmarshal(
		bytes.TrimSpace(output.Bytes()),
		&entry,
	); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	if entry["method"] != http.MethodGet {
		t.Fatalf(
			"expected method %q, got %v",
			http.MethodGet,
			entry["method"],
		)
	}

	if entry["path"] != "/health" {
		t.Fatalf(
			"expected path %q, got %v",
			"/health",
			entry["path"],
		)
	}

	status, ok := entry["status"].(float64)
	if !ok {
		t.Fatalf("expected numeric status, got %T", entry["status"])
	}

	if int(status) != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %v",
			http.StatusNoContent,
			status,
		)
	}

	if _, ok := entry["duration_ms"]; !ok {
		t.Fatal("expected duration_ms field")
	}

	if strings.Contains(output.String(), "secret") {
		t.Fatal("request log must not contain query parameters")
	}
}
