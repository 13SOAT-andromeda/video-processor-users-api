package middleware

import (
	"net/http"
	"strings"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/response"
	pkgjwt "github.com/13SOAT-andromeda/video-processor-users-api/pkg/jwt"
	"github.com/gin-gonic/gin"
)

const (
	ContextUserID    = "userID"
	ContextUserEmail = "email"
	ContextUserRole  = "role"
)

func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			response.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := pkgjwt.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			response.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
			c.Abort()
			return
		}

		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextUserEmail, claims.Email)
		c.Set(ContextUserRole, claims.Role)

		c.Next()
	}
}
