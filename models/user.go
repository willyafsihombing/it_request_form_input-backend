package models

import "time"

type User struct {
	ID           string     `json:"id"`
	EmailAddrs   string     `json:"emailAddrs"`
	UserName     string     `json:"userName"`
	FullName     string     `json:"fullName"`
	Role         string     `json:"role"`
	Department   string     `json:"department"`
	TitleUser    string     `json:"titleUser"`
	PasswordHash string     `json:"-"` // ← tidak pernah dikirim ke client
	DeletedAt    *time.Time `json:"deletedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// Response user — tanpa passwordHash
type UserResponse struct {
	ID         string     `json:"id"`
	EmailAddrs string     `json:"emailAddrs"`
	UserName   string     `json:"userName"`
	FullName   string     `json:"fullName"`
	Role       string     `json:"role"`
	Department string     `json:"department"`
	TitleUser  string     `json:"titleUser"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// Login request body
type LoginRequest struct {
	Login    string `json:"login" binding:"required"` // email atau username
	Password string `json:"password" binding:"required"`
}

// Login response
type LoginResponse struct {
	Success bool         `json:"success"`
	Token   string       `json:"token"`
	User    UserResponse `json:"user"`
}
