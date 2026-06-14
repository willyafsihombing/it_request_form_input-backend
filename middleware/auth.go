package middleware

import (
	"it-request-backend/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Cek header Authorization
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Cek cookie kalau header kosong
		if token == "" {
			token, _ = c.Cookie("auth-token")
		}

		if token == "" {
			c.JSON(401, gin.H{"error": "Unauthorized — token tidak ditemukan"})
			c.Abort()
			return
		}

		claims, err := utils.VerifyToken(token)
		if err != nil {
			c.JSON(401, gin.H{"error": "Unauthorized — token tidak valid"})
			c.Abort()
			return
		}

		if time.Now().After(claims.ExpiresAt.Time) {
			c.JSON(401, gin.H{
				"error": "Session expired",
				"code":  "TOKEN_EXPIRED",
			})
			c.Abort()
			c.Next()
		}

		c.Set("user", claims)
		c.Next()
	}
}
