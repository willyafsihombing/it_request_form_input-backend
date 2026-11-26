package middleware

import (
	"database/sql"
	"net/http"

	"it-request-backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

func LevelBlocked(db *sql.DB, userID string) (bool, error) {
	var level sql.NullString
	err := db.QueryRow(`
		SELECT e.level
		FROM users u
		JOIN employees e ON e.id::text = u.employee_id::text
		WHERE u.id::text = $1 AND u.deleted_at IS NULL`, userID).Scan(&level)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return utils.IsCreateBlockedLevel(level.String), nil
}

func BlockCreateByLevel(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := utils.CurrentClaims(c)
		if cl == nil {
			c.Next()
			return
		}
		blocked, err := LevelBlocked(db, cl.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "gagal memeriksa level pengguna"})
			return
		}
		if blocked {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Pengguna dengan level Manager tidak dapat membuat IT Request",
			})
			return
		}
		c.Next()
	}
}
