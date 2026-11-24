// handlers/stats_handler.go
package dashboard

import (
	"it-request-backend/internal/modules/itrequest"
	"it-request-backend/internal/shared/utils"
	"log"

	"github.com/gin-gonic/gin"
)

type ScopeFunc func(*gin.Context) (itrequest.ViewScope, bool)

type StatsHandler struct {
	Repo  *StatsRepository
	Scope ScopeFunc
}

func NewStatsHandler(repo *StatsRepository, scope ScopeFunc) *StatsHandler {
	return &StatsHandler{Repo: repo, Scope: scope}
}

func (h *StatsHandler) GetStats(c *gin.Context) {
	scope, ok := h.Scope(c)
	if !ok {
		return
	}

	stats, err := h.Repo.FetchStats(scope)
	if err != nil {
		log.Printf("Stats error : %v", err)
		utils.InternalError(c, "Internal server error")
		return
	}
	utils.OK(c, stats)
}
