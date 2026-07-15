package handlers

import (
	"fmt"
	"it-request-backend/models"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RequestHandler struct {
	Repo *repository.RequestRepository
}

func NewRequestHandler(repo *repository.RequestRepository) *RequestHandler {
	return &RequestHandler{Repo: repo}
}

// GET /api/requests
func (h *RequestHandler) GetAll(c *gin.Context) {
	status := c.Query("status")
	priority := c.Query("priority")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	reqName := c.Query("reqName")

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	requests, total, err := h.Repo.FindAll(status, priority, search, page, limit, reqName)
	if err != nil {
		utils.InternalError(c, "Internal server error")
		return
	}

	if requests == nil {
		requests = []models.RequestListItem{}
	}

	utils.OK(c, gin.H{
		"requests": requests,
		"total":    total,
		"page":     page,
		"limit":    limit,
		"pages":    int(math.Ceil(float64(total) / float64(limit))),
	})
}

// POST /api/requests

func (h *RequestHandler) Create(c *gin.Context) {
	var body models.Request
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Request body tidak valid")
		return
	}

	body.RequestTitle = strings.TrimSpace(body.RequestTitle)
	if body.RequestTitle == "" {
		utils.BadRequest(c, "Judul request wajib diisi")
		return
	}
	if len(body.RequestTitle) > 100 {
		utils.BadRequest(c, "Judul request maksimal 50 karakter")
		return
	}

	//sequence form number
	maxRetry := 5
	for i := 0; i < maxRetry; i++ {
		seq, err := utils.GenerateFormNumber(h.Repo.DB)
		if err != nil {
			utils.InternalError(c, "Gagal generate form number")
			return
		}
		body.FormNumber = utils.BuildFormNumber(seq, body.RequestTitle)

		exists, err := h.Repo.ExistsByFormNumber(body.FormNumber)
		if err != nil {
			utils.InternalError(c, "Internal server error")
			return
		}
		if !exists {
			break
		}
		if i == maxRetry-1 {
			utils.InternalError(c, "Gagal generate form number unik")
			return
		}
	}

	body.ID = uuid.New().String()
	body.Status = "pending"
	if body.Priority == "" {
		body.Priority = "normal"
	}
	if body.EffectiveDate == "" {
		body.EffectiveDate = time.Now().Format("1/2/2006")
	}

	if err := h.Repo.Create(&body); err != nil {
		fmt.Println("[Create Error]:", err.Error())
		utils.InternalError(c, "Gagal menyimpan request")
		return
	}

	utils.Created(c, body)
}

// GET /api/request/:id
func (h *RequestHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	req, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Internal Server Error")
		return
	}

	if req == nil {
		utils.NotFound(c, "Not Found")
		return
	}

	utils.OK(c, req)
}

// PATCH /api/request/:id
func (h *RequestHandler) UpdateByID(c *gin.Context) {
	id := c.Param("id")

	fmt.Println(id)

	exists, err := h.Repo.ExistsByID(id)
	if err != nil {
		utils.InternalError(c, "Internal Server Error")
		return
	}

	if !exists {
		utils.NotFound(c, "Request not found")
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Request body not valid")
		return
	}

	allowedFields := []string{
		"requestTitle", "status", "priority", "itNotes", "assignedTo",
		"techComment", "techAdd", "techAW", "techCR", "techEM", "techIP", "techEP",
		"signITAdmin", "signMSDIMgr", "signMgr", "signHOO", "costCode", "justification", "additionalDesc",
	}

	filtered := map[string]interface{}{}
	for _, field := range allowedFields {
		if val, ok := body[field]; ok {
			filtered[field] = val
		}
	}

	if err := h.Repo.Update(id, filtered); err != nil {
		utils.InternalError(c, "Wrong Request Upadate")
		return
	}

	updated, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Internal Server Error")
		return
	}

	utils.OK(c, updated)

}

func (h *RequestHandler) DeletedByID(c *gin.Context) {
	id := c.Param("id")

	if id == "" {
		utils.BadRequest(c, "ID Not Valid")
		return
	}

	exists, err := h.Repo.ExistsByID(id)
	if err != nil {
		utils.InternalError(c, "Internal Server Error")
		return
	}

	if !exists {
		utils.NotFound(c, "Request not found")
		return
	}

	if err := h.Repo.SoftDelete(id); err != nil {
		utils.InternalError(c, "Whats wrong deleted data")
		return
	}

	utils.OK(c, gin.H{"success": true})
}
