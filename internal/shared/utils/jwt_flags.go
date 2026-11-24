package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateTokenFlagged(id, email, username, fullName, role, department, title, employeeID, nik string, mustChangePassword bool) (string, error) {
	claims := JWTClaims{
		ID:                 id,
		EmailAddrs:         email,
		UserName:           username,
		FullName:           fullName,
		Role:               role,
		Department:         department,
		TitleUser:          title,
		EmployeeID:         employeeID,
		NIK:                nik,
		MustChangePassword: mustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL())),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
