package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultHTTPPort int    = 8080
	defaultLogLevel string = "info"
)

type Config struct {
	HTTPPort    int
	LogLevel    string
	DatabaseURL string
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

	return Config{
		HTTPPort:    port,
		LogLevel:    logLevel,
		DatabaseURL: databaseURL,
	}, nil
}
