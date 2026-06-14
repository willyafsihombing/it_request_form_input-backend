// handlers/stats_handler.go
package handlers

import (
	"it-request-backend/repository"
	"it-request-backend/utils"

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

	stats, err := h.Repo.FetchStats(reqName)
	if err != nil {
		// log.Printf("Stats error : %v%", err)
		utils.InternalError(c, "Internal server error")
		return
	}
	utils.OK(c, stats)
}
