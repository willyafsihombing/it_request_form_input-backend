// handlers/stats_handler.go
package handlers

import (
	"it-request-backend/repository"
	"it-request-backend/utils"
	"log"

	"github.com/gin-gonic/gin"
)

type StatsHandler struct {
	Repo *repository.StatsRepository
}

func NewStatsHandler(repo *repository.StatsRepository) *StatsHandler {
	return &StatsHandler{Repo: repo}
}

// GET /api/stats
func (h *StatsHandler) GetStats(c *gin.Context) {
	reqName := c.Query("reqName")
	createdBy := ""

	if claims := currentClaims(c); claims != nil && claims.Role == "user" {
		if claims.ID == "" {
			utils.Forbidden(c, "Akun tidak valid")
			return
		}
		reqName = ""
		createdBy = claims.ID
	}

	stats, err := h.Repo.FetchStats(reqName, createdBy)
	if err != nil {
		log.Printf("Stats error : %v", err)
		utils.InternalError(c, "Internal server error")
		return
	}
	utils.OK(c, stats)
}
