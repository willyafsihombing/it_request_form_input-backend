package handlers

import (
	"log"
	"strconv"
	"strings"

	"it-request-backend/models"
	"it-request-backend/utils"

	"github.com/gin-gonic/gin"
)

func (h *EmployeeHandler) searchScope(c *gin.Context) (dept string, ok bool) {
	cl := h.claims(c)
	if cl == nil {
		return "", false
	}
	if cl.Role == "admin" || cl.Role == "staff" {
		return "", true
	}

	if cl.EmployeeID != "" {
		if emp, err := h.Repo.FindByID(cl.EmployeeID); err == nil && emp != nil && emp.Departemen != "" {
			return emp.Departemen, true
		}
	}
	d := strings.TrimSpace(cl.Department)
	return d, d != ""
}

func (h *EmployeeHandler) SearchInScope(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	dept, ok := h.searchScope(c)
	if !ok {
		utils.OK(c, gin.H{"data": []models.EmployeeOption{}})
		return
	}

	list, err := h.Repo.SearchScoped(c.Query("q"), dept, limit)
	if err != nil {
		log.Println("employee search error:", err)
		utils.InternalError(c, "Gagal mencari karyawan")
		return
	}
	utils.OK(c, gin.H{"data": list})
}
