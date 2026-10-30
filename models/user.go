package models

import "time"

type User struct {
	ID                 string     `json:"id"`
	EmailAddrs         string     `json:"emailAddrs"`
	UserName           string     `json:"userName"`
	FullName           string     `json:"fullName"`
	Role               string     `json:"role"`
	Department         string     `json:"department"`
	TitleUser          string     `json:"titleUser"`
	EmployeeID         *string    `json:"employeeId"`
	MustChangePassword bool       `json:"mustChangePassword"`
	PasswordHash       string     `json:"-"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type UserResponse struct {
	ID                 string     `json:"id"`
	EmailAddrs         string     `json:"emailAddrs"`
	UserName           string     `json:"userName"`
	FullName           string     `json:"fullName"`
	Role               string     `json:"role"`
	Department         string     `json:"department"`
	TitleUser          string     `json:"titleUser"`
	EmployeeID         *string    `json:"employeeId"`
	MustChangePassword bool       `json:"mustChangePassword"`
	Employee           *Employee  `json:"employee"`
	passwordHash       string     `json:"-"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login response
type LoginResponse struct {
	Success bool         `json:"success"`
	Token   string       `json:"token"`
	User    UserResponse `json:"user"`
}
