package middleware

import (
	"net/http"
	"strings"

	"it-request-backend/utils"

	"github.com/gin-gonic/gin"
)

var mustChangeAllowed = []string{
	"/auth/change-password",
	"/auth/me",
	"/auth/logout",
}

func allowedWhileMustChange(path string) bool {
	for _, p := range mustChangeAllowed {
		if strings.HasSuffix(path, p) {
			return true
		}
	}
	return false
}

func MustChangeGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, _ := c.Get("user")
		claims, _ := v.(*utils.JWTClaims)
		if claims != nil && claims.MustChangePassword && !allowedWhileMustChange(c.Request.URL.Path) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Anda wajib mengganti password terlebih dahulu",
			})
			return
		}
		c.Next()
	}
}
