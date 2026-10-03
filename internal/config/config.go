package config

import (
	"fmt"
	"os"
	"strconv"
)

const defaultHTTPPort int = 8080

type Config struct {
	HTTPPort int
}

func Load() (Config, error) {
	port := defaultHTTPPort

	if value := os.Getenv("HTTP_PORT"); value != "" {
		parsedPort, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HTTP_PORT %q: %w", value, err)
		}

		if parsedPort < 1 || parsedPort > 65535 {
			return Config{}, fmt.Errorf("HTTP_PORT must be between 1 and 65535")
		}

		port = parsedPort
	}

	return Config{
		HTTPPort: port,
	}, nil
}
