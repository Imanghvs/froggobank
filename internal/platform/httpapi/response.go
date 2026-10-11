// Package httpapi provides shared HTTP response helpers for adapters.
package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error writes a public error message and aborts the remaining Gin handlers.
// The calling handler must still return after calling Error.
func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

func Unauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	Error(c, http.StatusUnauthorized, "unauthenticated")
}
