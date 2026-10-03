package server

import (
	"context"
	"log/slog"
	"net/http"

	servermiddleware "github.com/Imanghvs/froggobank/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func NewRouter(
	logger *slog.Logger,
	database DatabasePinger,
) *gin.Engine {
	router := gin.New()

	router.Use(servermiddleware.RequestLogger(logger))
	router.Use(gin.Recovery())

	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(
			c.Request.Context(),
			readinessTimeout,
		)
		defer cancel()

		err := database.Ping(ctx)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	return router
}
