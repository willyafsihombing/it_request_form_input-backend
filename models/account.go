package models

type AccountResult struct {
	EmployeeID string `json:"employeeId"`
	NIK        string `json:"nik"`
	Nama       string `json:"nama"`
	Departemen string `json:"departemen"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

type CreateAccountsRequest struct {
	EmployeeIDs []string `json:"employeeIds"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required"`
}
