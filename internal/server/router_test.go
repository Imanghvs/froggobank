package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := slog.New(
		slog.NewJSONHandler(io.Discard, nil),
	)

	router := NewRouter(logger, fakeDatabase{})

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

	router := NewRouter(logger, database)

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

	router := NewRouter(logger, database)

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

	router := NewRouter(logger, database)

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
