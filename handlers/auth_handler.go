package handlers

import (
	"it-request-backend/models"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	UserRepo *repository.UserRepository
}

func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{UserRepo: userRepo}
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var body models.LoginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Login dan password wajib diisi")
		return
	}

	login := strings.ToLower(strings.TrimSpace(body.Login))

	user, err := h.UserRepo.FindByLogin(login)
	if err != nil {
		utils.InternalError(c, "Internal server error")
		return
	}
	if user == nil {
		utils.BadRequest(c, "Email/username atau password salah")
		return
	}

	// Verifikasi password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		utils.BadRequest(c, "Email/username atau password salah")
		return
	}

	// Generate token
	token, err := utils.GenerateToken(
		user.ID, user.EmailAddrs, user.UserName,
		user.FullName, user.Role, user.Department, user.TitleUser,
	)
	if err != nil {
		utils.InternalError(c, "Gagal generate token")
		return
	}

	// Set cookie
	c.SetCookie("auth-token", token, 60*60*8, "/", "", false, true)

	utils.OK(c, models.LoginResponse{
		Success: true,
		Token:   token,
		User: models.UserResponse{
			ID:         user.ID,
			EmailAddrs: user.EmailAddrs,
			UserName:   user.UserName,
			FullName:   user.FullName,
			Role:       user.Role,
			Department: user.Department,
			TitleUser:  user.TitleUser,
			CreatedAt:  user.CreatedAt,
			UpdatedAt:  user.UpdatedAt,
		},
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

	utils.OK(c, gin.H{"user": models.UserResponse{
		ID:         user.ID,
		EmailAddrs: user.EmailAddrs,
		UserName:   user.UserName,
		FullName:   user.FullName,
		Role:       user.Role,
		Department: user.Department,
		TitleUser:  user.TitleUser,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}})
}
