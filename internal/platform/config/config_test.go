package config

import "testing"

const (
	portKey           string = "HTTP_PORT"
	logLevelKey       string = "LOG_LEVEL"
	databaseURLKey    string = "DATABASE_URL"
	sampleDatabaseURL string = "postgres://example"
)

func TestLoadUsesDefaultHTTPPort(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
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
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
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
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
	t.Setenv(portKey, "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadRejectsOutOfRangeHTTPPort(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
	t.Setenv(portKey, "666666")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadUsesDefaultLogLevel(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
	t.Setenv(logLevelKey, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Fatalf("expected log level %q, got %q", "info", cfg.LogLevel)
	}
}

func TestLoadUsesConfiguredLogLevel(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
	t.Setenv(logLevelKey, "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Fatalf("expected log level %q, got %q", "debug", cfg.LogLevel)
	}
}

func TestLoadRejectsEmptyDatabaseURL(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, "")
	t.Setenv(logLevelKey, "debug")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadUsesDatabaseURL(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
	t.Setenv(logLevelKey, "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DatabaseURL != sampleDatabaseURL {
		t.Fatalf("expected url %q, got %q", sampleDatabaseURL, cfg.DatabaseURL)
	}
}

func configureOIDC(t *testing.T) {
	t.Helper()
	t.Setenv("OIDC_ISSUER_URL", "https://identity.example/realms/test")
	t.Setenv("OIDC_AUDIENCE", "froggobank-api")
	t.Setenv("OIDC_ACCESS_TOKEN_PROFILE", "")
	t.Setenv("OIDC_ALLOW_INSECURE_HTTP", "")
}

func TestLoadOIDCConfiguration(t *testing.T) {
	configureOIDC(t)
	t.Setenv(databaseURLKey, sampleDatabaseURL)
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.OIDCIssuerURL != "https://identity.example/realms/test" || got.OIDCAudience != "froggobank-api" || got.OIDCAccessTokenProfile != "rfc9068" || got.OIDCAllowInsecureHTTP {
		t.Fatalf("invalid OIDC defaults: %+v", got)
	}
	t.Setenv("OIDC_ACCESS_TOKEN_PROFILE", "keycloak")
	t.Setenv("OIDC_ALLOW_INSECURE_HTTP", "true")
	got, err = Load()
	if err != nil || got.OIDCAccessTokenProfile != "keycloak" || !got.OIDCAllowInsecureHTTP {
		t.Fatalf("local OIDC config: %+v %v", got, err)
	}
}
func TestLoadRejectsMissingOrInvalidOIDCSettings(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"OIDC_ISSUER_URL", ""}, {"OIDC_AUDIENCE", ""}, {"OIDC_ACCESS_TOKEN_PROFILE", "unknown"}, {"OIDC_ALLOW_INSECURE_HTTP", "invalid"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			configureOIDC(t)
			t.Setenv(databaseURLKey, sampleDatabaseURL)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
