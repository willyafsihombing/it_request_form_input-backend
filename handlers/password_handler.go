package handlers

import (
	"log"
	"strings"
	"unicode"

	"it-request-backend/models"
	"it-request-backend/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// validateNewPassword mengembalikan pesan kesalahan, atau string kosong jika password baru memenuhi syarat.
func validateNewPassword(pw, username string) string {
	if len(pw) < 10 {
		return "Password baru minimal 10 karakter"
	}
	if len(pw) > 72 {
		return "Password baru maksimal 72 karakter"
	}
	hasLetter, hasDigit := false, false
	for _, r := range pw {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "Password baru harus mengandung huruf dan angka"
	}
	if strings.EqualFold(pw, username) {
		return "Password baru tidak boleh sama dengan username"
	}
	return ""
}

// POST /api/auth/change-password
// Body: {"currentPassword": "...", "newPassword": "..."}
// Berlaku untuk semua user yang login, termasuk yang wajib ganti password pertama kali.
// Setelah berhasil, token baru (tanpa penanda wajib ganti password) dikirim kembali.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	v, _ := c.Get("user")
	claims, _ := v.(*utils.JWTClaims)
	if claims == nil {
		utils.BadRequest(c, "Unauthorized")
		return
	}

	var body models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.BadRequest(c, "Password saat ini dan password baru wajib diisi")
		return
	}

	// Batasi percobaan menebak password saat ini
	key := "chg|" + claims.ID
	if blocked, wait := utils.LoginAttempts.Blocked(key); blocked {
		tooManyLogins(c, wait)
		return
	}

	user, err := h.UserRepo.FindByID(claims.ID)
	if err != nil || user == nil {
		utils.NotFound(c, "User tidak ditemukan")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.CurrentPassword)); err != nil {
		utils.LoginAttempts.Fail(key)
		utils.BadRequest(c, "Password saat ini salah")
		return
	}

	if msg := validateNewPassword(body.NewPassword, user.UserName); msg != "" {
		utils.BadRequest(c, msg)
		return
	}
	if body.NewPassword == body.CurrentPassword {
		utils.BadRequest(c, "Password baru tidak boleh sama dengan password saat ini")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), 12)
	if err != nil {
		utils.InternalError(c, "Gagal memproses password baru")
		return
	}
	if err := h.UserRepo.UpdatePassword(user.ID, string(hash), false); err != nil {
		log.Println("change password: UpdatePassword error:", err)
		utils.InternalError(c, "Gagal menyimpan password baru")
		return
	}
	utils.LoginAttempts.Reset(key)
	user.MustChangePassword = false

	emp, err := h.loadEmployee(user)
	if err != nil {
		log.Println("change password: loadEmployee error:", err)
		emp = nil
	}
	employeeID, nik := "", ""
	if emp != nil {
		employeeID, nik = emp.ID, emp.NIK
	}

	token, err := utils.GenerateTokenFlagged(
		user.ID, user.EmailAddrs, user.UserName,
		user.FullName, user.Role, user.Department, user.TitleUser, employeeID, nik, false,
	)
	if err != nil {
		log.Println("change password: GenerateTokenFlagged error:", err)
		utils.InternalError(c, "Password berhasil diganti, silakan login ulang")
		return
	}
	setAuthCookie(c, token, int(utils.TokenTTL().Seconds()))

	utils.OK(c, gin.H{
		"success": true,
		"token":   token,
		"user":    newUserResponse(user, emp),
	})
}
