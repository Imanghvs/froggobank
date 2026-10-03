package server

import (
	"log/slog"
	"net/http"

	servermiddleware "github.com/Imanghvs/froggobank/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func NewRouter(logger *slog.Logger) *gin.Engine {
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

	return router
}
