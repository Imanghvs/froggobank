package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Imanghvs/froggobank/internal/user/application"
)

func Me(c *gin.Context) {
	user, ok := application.UserFromContext(c.Request.Context())
	if !ok {
		c.Header("WWW-Authenticate", "Bearer")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	// The public profile contains no provider subject or credential material.
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "created_at": user.CreatedAt})
}
