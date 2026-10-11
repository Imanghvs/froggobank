package httpapi

import (
	"github.com/gin-gonic/gin"

	users "github.com/Imanghvs/froggobank/internal/user/application"
	userdomain "github.com/Imanghvs/froggobank/internal/user/domain"
)

func RequireUser(c *gin.Context) (userdomain.User, bool) {
	caller, ok := users.UserFromContext(c.Request.Context())
	if !ok {
		Unauthorized(c)
		return userdomain.User{}, false
	}
	return caller, true
}
