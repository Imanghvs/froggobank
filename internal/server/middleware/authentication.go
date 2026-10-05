package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Imanghvs/froggobank/internal/user/application"
	"github.com/Imanghvs/froggobank/internal/user/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AccessTokenVerifier interface {
	Verify(ctx context.Context, token string) (domain.Identity, error)
}

type UserResolver interface {
	ResolveIdentity(ctx context.Context, identity domain.Identity) (domain.User, error)
}

func Authentication(verifier AccessTokenVerifier, users UserResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		if len(values) != 1 {
			rejectCredentials(c)
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 16*1024 {
			rejectCredentials(c)
			return
		}
		identity, err := verifier.Verify(c.Request.Context(), parts[1])
		if err != nil || identity.Validate() != nil {
			rejectCredentials(c)
			return
		}
		user, err := users.ResolveIdentity(c.Request.Context(), identity)
		if err != nil || user.ID == uuid.Nil || user.Identity != identity {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve user"})
			return
		}
		c.Request = c.Request.WithContext(application.WithUser(c.Request.Context(), user))
		c.Next()
	}
}

func rejectCredentials(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
}
