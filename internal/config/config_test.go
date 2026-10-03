package config

import "testing"

const portKey string = "HTTP_PORT"

func TestLoadUsesDefaultHTTPPort(t *testing.T) {
	t.Setenv(portKey, "")
	cfg, err := Load()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Fatalf("expected default HTTP port %d, got %d", 8080, cfg.HTTPPort)
	}
}

func TestLoadUsesValidHTTPPort(t *testing.T) {
	t.Setenv(portKey, "9090")
	cfg, err := Load()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.HTTPPort != 9090 {
		t.Fatalf("expected set HTTP port %d, got %d", 9090, cfg.HTTPPort)
	}
}

func TestLoadRejectsNonIntHTTPPort(t *testing.T) {
	t.Setenv(portKey, "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadRejectsOutOfRangeHTTPPort(t *testing.T) {
	t.Setenv(portKey, "666666")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}
}
