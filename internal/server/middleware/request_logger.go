package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		logger.Info(
			"http request",
			slog.String("method", c.Request.Method),
			slog.String("path", route),
			slog.Int("status", c.Writer.Status()),
			slog.Int64(
				"duration_ms",
				time.Since(start).Milliseconds(),
			),
		)
	}
}
