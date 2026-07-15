package middleware

import (
	"net/http"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/response"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/gin-gonic/gin"
)

func RequireRole(required domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(ContextUserRole)
		if !exists {
			response.RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing identity claims")
			c.Abort()
			return
		}

		if domain.Role(role.(string)) != required {
			response.RespondError(c, http.StatusForbidden, "FORBIDDEN", "administrator role required")
			c.Abort()
			return
		}

		c.Next()
	}
}
