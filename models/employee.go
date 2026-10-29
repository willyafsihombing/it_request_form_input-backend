package models

import "time"

const (
	StaffTypeStaff    = "staff"
	StaffTypeNonStaff = "non_staff"
)

type Employee struct {
	ID         string     `json:"id"`
	NIK        string     `json:"nik"`
	Nama       string     `json:"nama"`
	Jabatan    string     `json:"jabatan"`
	Departemen string     `json:"departemen"`
	Level      string     `json:"level"`
	StaffType  string     `json:"staffType"`
	IsActive   bool       `json:"isActive"`
	DeletedAt  *time.Time `json:"deletedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func (e *Employee) IsStaff() bool {
	return e.StaffType == StaffTypeStaff
}

type EmployeeRequest struct {
	NIK        string `json:"nik" binding:"required"`
	Nama       string `json:"nama" binding:"required"`
	Jabatan    string `json:"jabatan"`
	Departemen string `json:"departemen"`
	Level      string `json:"level"`
	StaffType  string `json:"staffType" binding:"required,oneof=staff non_staff"`
	IsActive   *bool  `json:"isActive"`
}

type EmployeeOption struct {
	ID         string `json:"id"`
	NIK        string `json:"nik"`
	Nama       string `json:"nama"`
	Jabatan    string `json:"jabatan"`
	Departemen string `json:"departemen"`
	Level      string `json:"level"`
	StaffType  string `json:"staffType"`
}

type EmployeeStats struct {
	Total       int      `json:"total"`
	Active      int      `json:"active"`
	Inactive    int      `json:"inactive"`
	Staff       int      `json:"staff"`
	NonStaff    int      `json:"nonStaff"`
	Departments []string `json:"departments"`
}
