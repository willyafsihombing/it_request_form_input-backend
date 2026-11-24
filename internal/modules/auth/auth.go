package auth

import "it-request-backend/internal/modules/user"

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login response
type LoginResponse struct {
	Success bool              `json:"success"`
	Token   string            `json:"token"`
	User    user.UserResponse `json:"user"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}
