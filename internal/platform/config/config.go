package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHTTPPort int    = 8080
	defaultLogLevel string = "info"
)

type Config struct {
	HTTPPort               int
	LogLevel               string
	DatabaseURL            string
	OIDCIssuerURL          string
	OIDCAudience           string
	OIDCAccessTokenProfile string
	OIDCAllowInsecureHTTP  bool
}

func Load() (Config, error) {
	port := defaultHTTPPort

	if value := os.Getenv("HTTP_PORT"); value != "" {
		parsedPort, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_PORT %q: %w", value, err)
		}

		if parsedPort < 1 || parsedPort > 65535 {
			return Config{}, errors.New("HTTP_PORT must be between 1 and 65535")
		}

		port = parsedPort
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = defaultLogLevel
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	issuer := os.Getenv("OIDC_ISSUER_URL")
	if issuer == "" || strings.TrimSpace(issuer) != issuer {
		return Config{}, errors.New("OIDC_ISSUER_URL is required")
	}
	audience := os.Getenv("OIDC_AUDIENCE")
	if audience == "" || strings.TrimSpace(audience) != audience {
		return Config{}, errors.New("OIDC_AUDIENCE is required")
	}
	profile := os.Getenv("OIDC_ACCESS_TOKEN_PROFILE")
	if profile == "" {
		profile = "rfc9068"
	}
	if profile != "rfc9068" && profile != "keycloak" {
		return Config{}, errors.New("OIDC_ACCESS_TOKEN_PROFILE must be rfc9068 or keycloak")
	}
	allowHTTP := false
	if value := os.Getenv("OIDC_ALLOW_INSECURE_HTTP"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("OIDC_ALLOW_INSECURE_HTTP must be a boolean")
		}
		allowHTTP = parsed
	}

	return Config{
		HTTPPort:               port,
		LogLevel:               logLevel,
		DatabaseURL:            databaseURL,
		OIDCIssuerURL:          issuer,
		OIDCAudience:           audience,
		OIDCAccessTokenProfile: profile,
		OIDCAllowInsecureHTTP:  allowHTTP,
	}, nil
}
