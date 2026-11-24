package itrequest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"it-request-backend/internal/modules/approve"
	"it-request-backend/internal/modules/user"
	"it-request-backend/internal/shared/utils"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RequestHandler struct {
	Repo     *RequestRepository
	UserRepo *user.UserRepository

	Chains  *approve.ChainRepository
	Actions *approve.ActionRepository
}

func NewRequestHandler(repo *RequestRepository, userRepo *user.UserRepository) *RequestHandler {
	return &RequestHandler{Repo: repo, UserRepo: userRepo}
}

func (h *RequestHandler) UseApproval(chains *approve.ChainRepository, actions *approve.ActionRepository) *RequestHandler {
	h.Chains = chains
	h.Actions = actions
	return h
}

func ClearApproverSignatures(r *Request) {
	r.SigSptDept, r.SigDeptMgr, r.SigSrMgr, r.SigHOO = "", "", "", ""
	r.SigITAdmin, r.SigMSDIMgr = "", ""
}

func (h *RequestHandler) requestOrg(c *gin.Context, userID string) (empID, dept string, ok bool) {
	empID, dept, err := h.Repo.RequestOrg(userID)
	if err != nil {
		fmt.Println("[requestOrg Error]:", err)
		utils.InternalError(c, "Internal Server Error")
		return "", "", false
	}
	if empID == "" || dept == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Akun anda belum terhubung ke data karyawan atau departemen, hubungi ke administrator IT"})
		return "", "", false
	}
	return empID, dept, true
}

func chainProblem(c *gin.Context, err error) bool {
	var ce *approve.ChainError
	switch {
	case errors.As(err, &ce):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Alur persetujuan belum lengkap: " + strings.Join(ce.Problems, ";"),
			"problems": ce.Problems,
		})
		return true
	case errors.Is(err, approve.ErrNotRejected):
		c.JSON(http.StatusConflict, gin.H{"error": "Status request sudah berubah, muat ulang halaman"})
		return true
	}
	return false
}

func (h *RequestHandler) rebuildChain(ctx context.Context, tx *sql.Tx, id, dept, empID string, restart bool) error {
	status, err := h.Chains.LockRequestStatus(ctx, tx, id)
	if err != nil {
		return err
	}

	switch status {
	case approve.RequestRejected:
		if _, _, err := h.Chains.ResubmitChain(ctx, tx, id, dept, empID, restart); err != nil {
			return err
		}
		if restart && h.Actions != nil {
			if err := h.Actions.ClearSignatures(ctx, tx, id); err != nil {
				return err
			}
			return h.Actions.NotifyStepActivated(ctx, tx, id)
		}
		return nil
	case "":
		round, err := h.Chains.NextRound(ctx, tx, id)
		if err != nil {
			return err
		}
		if h.Actions != nil {
			if err := h.Actions.ClearSignatures(ctx, tx, id); err != nil {
				return err
			}
		}
		if _, err = h.Chains.InsertChain(ctx, tx, id, dept, empID, round); err != nil {
			return err
		}
		return h.Actions.NotifyStepActivated(ctx, tx, id)
	default:
		return approve.ErrNotRejected
	}

}

// GET /api/requests
func (h *RequestHandler) GetAll(c *gin.Context) {
	status := c.Query("status")
	priority := c.Query("priority")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	scope, ok := h.ResolveScope(c)
	if !ok {
		return
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	requests, total, err := h.Repo.FindAll(status, priority, search, page, limit, scope)
	if err != nil {
		utils.InternalError(c, "Internal server error")
		return
	}

	for i := range requests {
		requests[i].IsOwner = requests[i].CreatedByUserID != "" &&
			requests[i].CreatedByUserID == scope.UserID
	}

	if requests == nil {
		requests = []RequestListItem{}
	}

	utils.OK(c, gin.H{
		"requests": requests,
		"total":    total,
		"page":     page,
		"limit":    limit,
		"pages":    int(math.Ceil(float64(total) / float64(limit))),
	})
}

func (h *RequestHandler) ResolveScope(c *gin.Context) (ViewScope, bool) {
	cl := utils.CurrentClaims(c)
	if cl == nil || cl.ID == "" {
		utils.Forbidden(c, "Akun tidak valid")
		return ViewScope{}, false
	}
	sc := ViewScope{UserID: cl.ID}

	if cl.Role != "user" {
		sc.All = true
		return sc, true
	}

	empID, dept, err := h.Repo.RequestOrg(cl.ID)
	if err != nil {
		fmt.Println("[viewScope Error]:", err)
		utils.InternalError(c, "Internal server error")
		return ViewScope{}, false
	}
	sc.EmployeeID, sc.Dept = empID, dept

	if h.Chains != nil && empID != "" {
		if yes, _ := h.Chains.IsDesignated(c.Request.Context(), empID, approve.StepITManager); yes {
			sc.All = true
		}
	}
	return sc, true
}

// POST /api/requests
func (h *RequestHandler) Create(c *gin.Context) {
	var body Request
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

	body.CreatedByUserID = nil
	if claims := utils.CurrentClaims(c); claims != nil && claims.ID != "" {
		uid := claims.ID
		body.CreatedByUserID = &uid
	}

	body.ID = uuid.New().String()
	body.Status = "pending"
	if body.Priority == "" {
		body.Priority = "normal"
	}
	if body.EffectiveDate == "" {
		body.EffectiveDate = time.Now().Format("1/2/2006")
	}

	var createErr error
	if h.Chains != nil {
		ClearApproverSignatures(&body)

		uid := ""
		if body.CreatedByUserID != nil {
			uid = *body.CreatedByUserID
		}
		empID, dept, ok := h.requestOrg(c, uid)
		if !ok {
			return
		}
		createErr = h.Repo.CreateInTx(&body, func(tx *sql.Tx) error {
			if _, err := h.Chains.InsertChain(c.Request.Context(), tx, body.ID, dept, empID, 1); err != nil {
				return err
			}
			return h.Actions.NotifyStepActivated(c.Request.Context(), tx, body.ID)
		})
	} else {
		createErr = h.Repo.Create(&body)
	}
	if createErr != nil {
		if chainProblem(c, createErr) {
			return
		}
		fmt.Println("[Create Error]:", createErr.Error())
		utils.InternalError(c, "Gagal menyimpan request")
		return
	}

	utils.Created(c, body)
}

// GET /api/request/:id
func (h *RequestHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	if utils.CurrentClaims(c) != nil {
		scope, ok := h.ResolveScope(c)
		if !ok {
			return
		}
		okView, err := h.Repo.CanView(id, scope)
		if err != nil {
			utils.InternalError(c, "Internal Server Error")
			return
		}
		if !okView {
			utils.NotFound(c, "Not Found")
			return
		}
	}

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

	existing, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Internal Server Error")
		return
	}

	if existing == nil {
		utils.NotFound(c, "Request not found")
		return
	}

	if claims := utils.CurrentClaims(c); claims != nil {
		if claims.Role == "staff" && existing.Status == "approved" {
			utils.Forbidden(c, "Request yang sudah approved tidak bisa diubah. Hubungi admin.")
			return
		}
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Request body not valid")
		return
	}

	allowedFields := []string{
		"requestTitle", "status", "priority", "itNotes", "assignedTo",
		"techComment", "techAD", "techAW", "techCR", "techEM", "techIP", "techEP",
		"sigITAdmin", "sigMSDIMgr", "sigSrMgr", "sigHOO", "costCode", "justification", "additionalDesc",
	}

	filtered := map[string]interface{}{}
	for _, field := range allowedFields {
		if val, ok := body[field]; ok {
			filtered[field] = val
		}
	}

	if existing.ApprovalStatus != "" {
		for _, f := range []string{"sigITAdmin", "sigMSDIMgr", "sigSrMgr", "sigHOO"} {
			if _, ok := filtered[f]; ok {
				utils.Forbidden(c, "Tanda tangan pejabat hanya terisi lewat tombol approve")
				return
			}
		}
		if v, ok := filtered["status"]; ok {
			if s, _ := v.(string); s == "pending" || s == "approved" || s == "rejected" {
				utils.Forbidden(c, "Status ini hanya berubah melalui proses persetujuan")
				return
			}
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

func (h *RequestHandler) Resubmit(c *gin.Context) {
	id := c.Param("id")
	claims := c.MustGet("user").(*utils.JWTClaims)

	var body Request
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
		utils.BadRequest(c, "Judul request maksimal 100 karakter")
		return
	}
	existing, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Internal server error")
		return
	}
	if existing == nil {
		utils.NotFound(c, "Request tidak ditemukan")
		return
	}
	if existing.Status != "rejected" {
		c.JSON(http.StatusConflict, gin.H{"error": "Hanya request yang ditolak yang dapat diedit"})
		return
	}
	if claims.Role == "user" && !h.isOwner(existing, claims) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda hanya dapat mengedit request milik Anda"})
		return
	}

	if h.Chains != nil {
		ClearApproverSignatures(&body)
	}

	body.ID = id
	body.Status = "pending"
	body.ReqName = existing.ReqName
	body.SigRequester = existing.SigRequester
	if body.Priority == "" {
		body.Priority = "normal"
	}

	var ok bool
	if h.Chains != nil {
		creator := claims.ID
		if existing.CreatedByUserID != nil {
			creator = *existing.CreatedByUserID
		}
		empID, dept, good := h.requestOrg(c, creator)
		if !good {
			return
		}
		restart := c.Query("restart") == "true"
		ok, err = h.Repo.ResubmitInTx(id, &body, func(tx *sql.Tx) error {
			return h.rebuildChain(c.Request.Context(), tx, id, dept, empID, restart)
		})
	} else {
		ok, err = h.Repo.Resubmit(id, &body)
	}
	if err != nil {
		if chainProblem(c, err) {
			return
		}
		utils.InternalError(c, "Gagal menyimpan perubahan")
		return
	}

	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "Status request sudah berubah, muat ulang halaman"})
		return
	}

	updated, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Internal server error")
		return
	}
	utils.OK(c, updated)
}

func (h *RequestHandler) isOwner(r *Request, claims *utils.JWTClaims) bool {
	return r.CreatedByUserID != nil && claims != nil && *r.CreatedByUserID == claims.ID
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

	if cur, err := h.Repo.FindByID(id); err == nil && cur != nil && cur.ApprovalStatus == string(approve.RequestInReview) {
		c.JSON(http.StatusConflict, gin.H{"error": "Request sedang dalam proses persetujuan, tidak bisa dihapus"})
		return
	}

	if err := h.Repo.SoftDelete(id); err != nil {
		utils.InternalError(c, "Whats wrong deleted data")
		return
	}

	utils.OK(c, gin.H{"success": true})
}
