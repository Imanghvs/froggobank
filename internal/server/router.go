package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Imanghvs/froggobank/internal/account/adapters/httpapi"
	servermiddleware "github.com/Imanghvs/froggobank/internal/server/middleware"
	userhttp "github.com/Imanghvs/froggobank/internal/user/adapters/httpapi"
)

func NewRouter(
	logger *slog.Logger,
	database DatabasePinger,
	accountHandler *httpapi.Handler,
	verifier servermiddleware.AccessTokenVerifier,
	users servermiddleware.UserResolver,
) *gin.Engine {
	router := gin.New()

	router.Use(servermiddleware.RequestLogger(logger))
	router.Use(gin.Recovery())

	if err := router.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	registerDocumentation(router)

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

	protected := router.Group("", servermiddleware.Authentication(verifier, users))
	protected.GET("/me", userhttp.Me)
	protected.POST("/accounts", accountHandler.Create)
	protected.GET("/accounts/:id", accountHandler.GetByID)
	protected.GET("/accounts/:id/balance", accountHandler.GetBalance)
	protected.GET("/accounts", accountHandler.GetUserAccounts)

	return router
}
