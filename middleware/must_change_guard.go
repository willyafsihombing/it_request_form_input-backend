package middleware

import (
	"it-request-backend/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func MustChangeGuard() gin.HandlerFunc {
	allowed := map[string]bool{
		"api/auth/change-password": true,
		"api/auth/me":              true,
		"api/auth/logout":          true,
	}
	return func(c *gin.Context) {
		v, _ := c.Get("user")
		claims, _ := v.(*utils.JWTClaims)
		if claims != nil && claims.MustChangePassword && !allowed[c.FullPath()] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Anda wajib mengganti password terlebih dahulu",
			})
			return
		}
		c.Next()
	}
}
