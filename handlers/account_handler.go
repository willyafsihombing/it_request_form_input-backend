package handlers

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"runtime"
	"strings"
	"sync"

	"it-request-backend/models"
	"it-request-backend/repository"
	"it-request-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxAccountsPerRequest = 100
	maxPendingIDs         = 5000
	generatedPasswordLen  = 12

	passwordUpper    = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	passwordLower    = "abcdefghijkmnopqrstuvwxyz"
	passwordDigits   = "23456789"
	passwordAlphabet = passwordUpper + passwordLower + passwordDigits
)

type AccountHandler struct {
	EmployeeRepo *repository.EmployeeRepository
	UserRepo     *repository.UserRepository
}

func NewAccountHandler(employeeRepo *repository.EmployeeRepository, userRepo *repository.UserRepository) *AccountHandler {
	return &AccountHandler{EmployeeRepo: employeeRepo, UserRepo: userRepo}
}

// Hanya admin yang boleh membuat akun dan mereset password.
func (h *AccountHandler) requireAdmin(c *gin.Context) (*utils.JWTClaims, bool) {
	v, _ := c.Get("user")
	cl, _ := v.(*utils.JWTClaims)
	if cl == nil || cl.Role != "admin" {
		utils.Forbidden(c, "Hanya admin yang dapat mengelola akun karyawan")
		return nil, false
	}
	return cl, true
}

func cleanIDs(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		id := u.String()
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func generatePassword(n int) (string, error) {
	for {
		b := make([]byte, n)
		for i := range b {
			idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
			if err != nil {
				return "", err
			}
			b[i] = passwordAlphabet[idx.Int64()]
		}
		s := string(b)
		if strings.ContainsAny(s, passwordUpper) && strings.ContainsAny(s, passwordLower) && strings.ContainsAny(s, passwordDigits) {
			return s, nil
		}
	}
}

// GET /api/employees/accounts/pending?departemen=
func (h *AccountHandler) AccountsPending(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}

	ids, err := h.EmployeeRepo.IDsWithoutAccount(c.Query("departemen"), maxPendingIDs)
	if err != nil {
		log.Println("accounts pending error:", err)
		utils.InternalError(c, "Gagal memuat daftar karyawan")
		return
	}
	utils.OK(c, gin.H{"ids": ids, "total": len(ids)})
}

// POST /api/employees/accounts/status
func (h *AccountHandler) AccountStatus(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}

	var body struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Format permintaan tidak valid")
		return
	}
	ids := cleanIDs(body.IDs)
	if len(ids) > 300 {
		utils.BadRequest(c, "Terlalu banyak id dalam satu permintaan")
		return
	}

	accounts, err := h.UserRepo.AccountsByEmployeeIDs(ids)
	if err != nil {
		log.Println("account status error:", err)
		utils.InternalError(c, "Gagal memuat status akun")
		return
	}
	utils.OK(c, gin.H{"accounts": accounts})
}

type accountCandidate struct {
	emp      models.Employee
	username string
	email    string
	base     models.AccountResult
	password string
	hash     []byte
	err      error
}

// POST /api/employees/accounts
func (h *AccountHandler) CreateAccounts(c *gin.Context) {
	if _, ok := h.requireAdmin(c); !ok {
		return
	}

	var body models.CreateAccountsRequest
	if err := c.ShouldBindJSON(&body); err != nil || len(body.EmployeeIDs) == 0 {
		utils.BadRequest(c, "Pilih minimal satu karyawan")
		return
	}
	ids := cleanIDs(body.EmployeeIDs)
	if len(ids) == 0 {
		utils.BadRequest(c, "ID karyawan tidak valid")
		return
	}
	if len(ids) > maxAccountsPerRequest {
		utils.BadRequest(c, fmt.Sprintf("Maksimal %d karyawan per permintaan", maxAccountsPerRequest))
		return
	}

	emps, err := h.EmployeeRepo.FindActiveByIDs(ids)
	if err != nil {
		log.Println("create accounts: FindActiveByIDs error:", err)
		utils.InternalError(c, "Gagal memuat data karyawan")
		return
	}
	found := make(map[string]models.Employee, len(emps))
	for _, e := range emps {
		found[e.ID] = e
	}

	existing, err := h.UserRepo.AccountsByEmployeeIDs(ids)
	if err != nil {
		log.Println("create accounts: AccountsByEmployeeIDs error:", err)
		utils.InternalError(c, "Gagal memeriksa akun yang sudah ada")
		return
	}

	results := make([]models.AccountResult, 0, len(ids))
	var cands []*accountCandidate
	var usernames, emails []string

	for _, id := range ids {
		emp, ok := found[id]
		if !ok {
			results = append(results, models.AccountResult{
				EmployeeID: id, Status: "skipped", Message: "Karyawan tidak ditemukan atau nonaktif",
			})
			continue
		}

		base := models.AccountResult{
			EmployeeID: emp.ID, NIK: emp.NIK, Nama: emp.Nama, Departemen: emp.Departemen,
		}
		if uname, has := existing[id]; has {
			base.Username, base.Status, base.Message = uname, "skipped", "Sudah punya akun"
			results = append(results, base)
			continue
		}

		username := strings.ToLower(strings.TrimSpace(emp.NIK))
		email := username + "@unggul.com"
		cands = append(cands, &accountCandidate{emp: emp, username: username, email: email, base: base})
		usernames = append(usernames, username)
		emails = append(emails, email)
	}

	takenUsers, takenEmails, err := h.UserRepo.ExistingLoginNames(usernames, emails)
	if err != nil {
		log.Println("create accounts: ExistingLoginNames error:", err)
		utils.InternalError(c, "Gagal memeriksa username yang sudah ada")
		return
	}

	var ready []*accountCandidate
	for _, cd := range cands {
		if takenUsers[cd.username] || takenEmails[cd.email] {
			r := cd.base
			r.Username, r.Status, r.Message = cd.username, "failed", "Username (NIK) sudah dipakai akun lain"
			results = append(results, r)
			continue
		}
		ready = append(ready, cd)
	}

	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, cd := range ready {
		wg.Add(1)
		sem <- struct{}{}
		go func(cd *accountCandidate) {
			defer wg.Done()
			defer func() { <-sem }()

			pw, err := generatePassword(generatedPasswordLen)
			if err != nil {
				cd.err = err
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
			if err != nil {
				cd.err = err
				return
			}
			cd.password, cd.hash = pw, hash
		}(cd)
	}
	wg.Wait()

	created, skipped, failed := 0, 0, 0
	for _, cd := range ready {
		r := cd.base
		r.Username = cd.username

		if cd.err != nil {
			log.Println("create accounts: hash/generate error:", cd.err)
			r.Status, r.Message = "failed", "Gagal membuat password"
			results = append(results, r)
			continue
		}

		err := h.UserRepo.InsertAccount(repository.NewAccount{
			EmployeeID:   cd.emp.ID,
			Email:        cd.email,
			UserName:     cd.username,
			FullName:     cd.emp.Nama,
			Role:         "user",
			Department:   cd.emp.Departemen,
			Title:        cd.emp.Jabatan,
			PasswordHash: string(cd.hash),
		})
		switch {
		case err == repository.ErrAccountDuplicate:
			r.Status, r.Message = "failed", "Username atau email sudah dipakai"
		case err != nil:
			log.Println("create accounts: InsertAccount error:", err)
			r.Status, r.Message = "failed", "Gagal menyimpan akun"
		default:
			r.Status, r.Password = "created", cd.password
		}
		results = append(results, r)
	}

	for _, r := range results {
		switch r.Status {
		case "created":
			created++
		case "skipped":
			skipped++
		default:
			failed++
		}
	}

	utils.OK(c, gin.H{"results": results, "created": created, "skipped": skipped, "failed": failed})
}

// POST /api/employees/:id/reset-password
func (h *AccountHandler) ResetPassword(c *gin.Context) {
	cl, ok := h.requireAdmin(c)
	if !ok {
		return
	}

	u, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "ID karyawan tidak valid")
		return
	}
	id := u.String()

	if cl.EmployeeID == id {
		utils.BadRequest(c, "Gunakan menu ganti password untuk akun Anda sendiri")
		return
	}

	emp, err := h.EmployeeRepo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Gagal memuat data karyawan")
		return
	}
	if emp == nil {
		utils.NotFound(c, "Karyawan tidak ditemukan")
		return
	}

	user, err := h.UserRepo.FindByEmployeeID(id)
	if err != nil {
		utils.InternalError(c, "Gagal memuat akun")
		return
	}
	if user == nil {
		utils.NotFound(c, "Karyawan ini belum punya akun")
		return
	}

	pw, err := generatePassword(generatedPasswordLen)
	if err != nil {
		utils.InternalError(c, "Gagal membuat password")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	if err != nil {
		utils.InternalError(c, "Gagal membuat password")
		return
	}
	if err := h.UserRepo.UpdatePassword(user.ID, string(hash), true); err != nil {
		log.Println("reset password: UpdatePassword error:", err)
		utils.InternalError(c, "Gagal menyimpan password baru")
		return
	}

	utils.LoginAttempts.ResetPrefix(strings.ToLower(user.UserName) + "|")

	utils.OK(c, gin.H{"result": models.AccountResult{
		EmployeeID: emp.ID,
		NIK:        emp.NIK,
		Nama:       emp.Nama,
		Departemen: emp.Departemen,
		Username:   user.UserName,
		Password:   pw,
		Status:     "reset",
	}})
}
