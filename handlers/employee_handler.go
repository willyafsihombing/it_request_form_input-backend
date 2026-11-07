package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"it-request-backend/models"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

const (
	maxImportRows      = 5000
	maxImportBodyBytes = 5 << 20
)

type EmployeeHandler struct {
	Repo *repository.EmployeeRepository
}

func NewEmployeeHandler(repo *repository.EmployeeRepository) *EmployeeHandler {
	return &EmployeeHandler{Repo: repo}
}

func (h *EmployeeHandler) claims(c *gin.Context) *utils.JWTClaims {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	cl, _ := v.(*utils.JWTClaims)
	return cl
}

func (h *EmployeeHandler) deny(c *gin.Context, msg string) {
	utils.Forbidden(c, msg)
}

func (h *EmployeeHandler) requireView(c *gin.Context) bool {
	cl := h.claims(c)
	if cl == nil || (cl.Role != "admin" && cl.Role != "staff") {
		h.deny(c, "Anda tidak memiliki akses ke data karyawan")
		return false
	}
	return true
}

func (h *EmployeeHandler) requireManage(c *gin.Context) bool {
	cl := h.claims(c)
	if cl == nil || cl.Role != "admin" {
		h.deny(c, "Hanya admin yang dapat mengelola data karyawan")
		return false
	}
	return true
}

func parseEmployeeID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		utils.BadRequest(c, "ID karyawan tidak valid")
		return "", false
	}
	return id, true
}

func normalizeStaffType(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)
	switch s {
	case "staff":
		return models.StaffTypeStaff
	case "non_staff", "nonstaff":
		return models.StaffTypeNonStaff
	}
	return ""
}

func (h *EmployeeHandler) Search(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	list, err := h.Repo.Search(c.Query("q"), limit)
	if err != nil {
		utils.InternalError(c, "Gagal mencari karyawan")
		return
	}
	utils.OK(c, gin.H{"data": list})
}

func (h *EmployeeHandler) GetAll(c *gin.Context) {
	if !h.requireView(c) {
		return
	}

	status := c.Query("status")
	staffType := c.Query("staffType")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}

	account := c.Query("account")
	list, total, err := h.Repo.List(c.Query("q"), c.Query("departemen"), status, staffType, account, page, perPage)
	if err != nil {
		utils.InternalError(c, "Gagal memuat data karyawan")
		return
	}

	utils.OK(c, gin.H{
		"data":    list,
		"total":   total,
		"page":    page,
		"perPage": perPage,
	})
}

func (h *EmployeeHandler) GetByID(c *gin.Context) {
	if !h.requireView(c) {
		return
	}
	id, ok := parseEmployeeID(c)
	if !ok {
		return
	}

	emp, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Gagal membuat data karyawan")
		return
	}
	if emp == nil {
		utils.InternalError(c, "Karyawan tidak ditemukan")
		return
	}
	utils.OK(c, gin.H{"employee": emp})
}

func (h *EmployeeHandler) Create(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}

	var body models.EmployeeRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "NIK, nama, dan tipe(staff / non staff) wajib diisi")
		return
	}

	emp, err := h.Repo.Create(body)
	if errors.Is(err, repository.ErrEmployeeNIKExists) {
		utils.Conflict(c, err.Error())
		return
	}
	if err != nil {
		log.Println("Create employee error", err)
		utils.InternalError(c, "Gagal menambah karyawan")
		return
	}
	utils.Created(c, gin.H{"employee": emp})
}

func (h *EmployeeHandler) Update(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	id, ok := parseEmployeeID(c)
	if !ok {
		return
	}

	var body models.EmployeeRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "NIK, nama, dan tipe (staff / non_staff) wajib diisi")
		return
	}

	emp, err := h.Repo.Update(id, body)
	if errors.Is(err, repository.ErrEmployeeNIKExists) {
		utils.Conflict(c, err.Error())
		return
	}
	if err != nil {
		utils.InternalError(c, "Gagal memperbarui karyawan")
		return
	}
	if emp == nil {
		utils.NotFound(c, "Karyawan tidak ditemukan")
		return
	}
	utils.OK(c, gin.H{"employee": emp})
}

func (h *EmployeeHandler) Delete(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	id, ok := parseEmployeeID(c)
	if !ok {
		return
	}

	if cl := h.claims(c); cl != nil && cl.EmployeeID == id {
		utils.BadRequest(c, "Tidak dapat menghapus data karyawan milik akun Anda sendiri")
		return
	}

	deleted, err := h.Repo.SoftDelete(id)
	if err != nil {
		utils.InternalError(c, "Gagal menghapus karyawan")
		return
	}
	if !deleted {
		utils.NotFound(c, "Karyawan tidak ditemukan")
		return
	}
	utils.OK(c, gin.H{"success": true})
}

func (h *EmployeeHandler) Import(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportBodyBytes)

	var rows []models.EmployeeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&rows); err != nil {
		utils.BadRequest(c, "Format data impor tidak valid (harus berupa array karyawan)")
		return
	}
	if len(rows) == 0 {
		utils.BadRequest(c, "Tidak ada data untuk diimpor")
		return
	}
	if len(rows) > maxImportRows {
		utils.BadRequest(c, fmt.Sprintf("Maksimal %d baris per impor", maxImportRows))
		return
	}

	var problems []string
	for i := range rows {
		rows[i].NIK = strings.TrimSpace(rows[i].NIK)
		rows[i].Nama = strings.TrimSpace(rows[i].Nama)
		rows[i].StaffType = normalizeStaffType(rows[i].StaffType)

		n := i + 1
		switch {
		case rows[i].NIK == "":
			problems = append(problems, fmt.Sprintf("Baris %d: NIK kosong", n))
		case rows[i].Nama == "":
			problems = append(problems, fmt.Sprintf("Baris %d: nama kosong", n))
		case rows[i].StaffType == "":
			problems = append(problems, fmt.Sprintf("Baris %d: tipe harus Staff atau Non Staff", n))
		}
	}
	if len(problems) > 0 {
		shown := problems
		if len(shown) > 10 {
			shown = shown[:10]
		}
		utils.BadRequest(c, fmt.Sprintf(
			"Impor dibatalkan, %d baris bermasalah. %s",
			len(problems), strings.Join(shown, "; "),
		))
		return
	}

	n, err := h.Repo.UpsertMany(rows)
	if err != nil {
		utils.InternalError(c, "Gagal mengimpor data karyawan")
		return
	}
	utils.OK(c, gin.H{"success": true, "imported": n})
}

func (h *EmployeeHandler) Stats(c *gin.Context) {
	if !h.requireView(c) {
		return
	}
	s, err := h.Repo.Stats()
	if err != nil {
		log.Println("employee stats error:", err)
		utils.InternalError(c, "Gagal memuat statistik karyawan")
		return
	}
	utils.OK(c, s)
}
