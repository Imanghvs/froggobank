package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestNewRejectsInvalidLogLevel(t *testing.T) {
	_, err := New("verbose", io.Discard)

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoggerRespectsConfiguredLevel(t *testing.T) {
	var output bytes.Buffer

	logger, err := New("warn", &output)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logger.Info("hidden message")
	logger.Warn("visible message")

	raw := bytes.TrimSpace(output.Bytes())

	if !json.Valid(raw) {
		t.Fatalf("expected valid JSON log, got %q", string(raw))
	}

	if strings.Contains(string(raw), "hidden message") {
		t.Fatal("expected info log to be filtered")
	}

	if !strings.Contains(string(raw), "visible message") {
		t.Fatal("expected warning log to be emitted")
	}
}
