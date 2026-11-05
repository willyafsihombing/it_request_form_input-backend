package middleware

import (
	"database/sql"
	"errors"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SessionGuard(repo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, _ := c.Get("user")
		claims, _ := v.(*utils.JWTClaims)
		if claims == nil {
			c.Next()
			return
		}

		changedAt, err := repo.PasswordChangedAt(claims.ID)
		if errors.Is(err, sql.ErrNoRows) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sesi tidak valid"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		if claims.IssuedAt != nil && claims.IssuedAt.Unix() < changedAt.Unix() {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Password telah diganti, silakan login ulang"})
			return
		}
		c.Next()
	}
}
