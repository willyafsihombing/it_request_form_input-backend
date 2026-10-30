package handlers

import (
	"fmt"
	"it-request-backend/models"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	UserRepo     *repository.UserRepository
	EmployeeRepo *repository.EmployeeRepository
}

func NewAuthHandler(userRepo *repository.UserRepository, employeeRepo *repository.EmployeeRepository) *AuthHandler {
	return &AuthHandler{
		UserRepo:     userRepo,
		EmployeeRepo: employeeRepo,
	}
}

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), 12)

func setAuthCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "auth-token",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   os.Getenv("COOKIE_SECURE") == "true",
		SameSite: http.SameSiteLaxMode,
	})
}

func tooManyLogins(c *gin.Context, wait time.Duration) {
	c.Header("Retry-After", strconv.Itoa(int(wait.Seconds())))
	utils.TooManyRequests(c, fmt.Sprintf(
		"Terlalu banyak percobaan login. Coba lagi dalam %d menit", int(wait.Minutes())+1))
}

func failLogin(c *gin.Context, userKey, ip string) {
	utils.LoginAttempts.Fail(userKey)
	utils.IPAttempts.Fail(ip)
	utils.BadRequest(c, "Email/username atau password salah")
}

func (h *AuthHandler) loadEmployee(user *models.User) (*models.Employee, error) {
	if user.EmployeeID == nil || *user.EmployeeID == "" {
		return nil, nil
	}
	return h.EmployeeRepo.FindByID(*user.EmployeeID)
}

func newUserResponse(u *models.User, emp *models.Employee) models.UserResponse {
	return models.UserResponse{
		ID:         u.ID,
		EmailAddrs: u.EmailAddrs,
		UserName:   u.UserName,
		FullName:   u.FullName,
		Role:       u.Role,
		Department: u.Department,
		TitleUser:  u.TitleUser,
		EmployeeID: u.EmployeeID,
		Employee:   emp,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var body models.LoginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Login dan password wajib diisi")
		return
	}

	login := strings.ToLower(strings.TrimSpace(body.Login))
	ip := c.ClientIP()
	userKey := login + "|" + ip

	if blocked, wait := utils.LoginAttempts.Blocked(userKey); blocked {
		tooManyLogins(c, wait)
		return
	}

	if blocked, wait := utils.IPAttempts.Blocked(ip); blocked {
		tooManyLogins(c, wait)
		return
	}

	user, err := h.UserRepo.FindByLogin(login)
	if err != nil {
		utils.InternalError(c, "Internal server error")
		return
	}
	if user == nil {
		utils.BadRequest(c, "Email/username atau password salah")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		failLogin(c, userKey, ip)
		return
	}

	utils.LoginAttempts.Reset(userKey)

	emp, err := h.loadEmployee(user)
	if err != nil {
		log.Println("Login: loademployee error:", err)
		utils.InternalError(c, "Gagal memuat data karyawan")
		return
	}

	if user.EmployeeID != nil && *user.EmployeeID != "" && (emp == nil || !emp.IsActive) {
		utils.BadRequest(c, "Akun karyawan tidak aktif. Silakan hubungi HRD untuk mengaktifkan kembali akun Anda.")
		return
	}

	employeeID, nik := "", ""
	if emp != nil {
		employeeID, nik = emp.ID, emp.NIK
	}

	token, err := utils.GenerateToken(
		user.ID, user.EmailAddrs, user.UserName,
		user.FullName, user.Role, user.Department, user.TitleUser, employeeID, nik,
	)
	if err != nil {
		log.Println("login: GenerateToken error", err)
		utils.InternalError(c, "Gagal generate token")
		return
	}

	setAuthCookie(c, token, int(utils.TokenTTL().Seconds()))

	utils.OK(c, models.LoginResponse{
		Success: true,
		Token:   token,
		User:    newUserResponse(user, emp),
	})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("auth-token", "", -1, "/", "", false, true)
	utils.OK(c, gin.H{"success": true})
}

// GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	claims, exists := c.Get("user")
	if !exists {
		utils.BadRequest(c, "Unauthorized")
		return
	}

	jwtClaims := claims.(*utils.JWTClaims)

	user, err := h.UserRepo.FindByID(jwtClaims.ID)
	if err != nil || user == nil {
		utils.NotFound(c, "User tidak ditemukan")
		return
	}

	emp, err := h.loadEmployee(user)
	if err != nil {
		utils.InternalError(c, "Gagal memuat data karyawan")
		return
	}

	utils.OK(c, gin.H{"user": newUserResponse(user, emp)})
}
