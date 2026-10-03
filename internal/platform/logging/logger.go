package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func New(levelName string, output io.Writer) (*slog.Logger, error) {
	level, err := parseLevel(levelName)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level})

	return slog.New(handler), nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf(
			"invalid log level %q: expected debug, info, warn, or error",
			value,
		)
	}
}
