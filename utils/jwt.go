package utils

import (
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	ID         string `json:"id"`
	EmailAddrs string `json:"emailAddrs"`
	UserName   string `json:"userName"`
	FullName   string `json:"fullName"`
	Role       string `json:"role"`
	Department string `json:"department"`
	TitleUser  string `json:"titleUser"`
	EmployeeID string `json:"employeeId,omitempty"`
	NIK        string `json:"nik,omitempty"`
	jwt.RegisteredClaims
}

func GenerateToken(id, email, username, fullName, role, department, title, employeeID, nik string) (string, error) {
	claims := JWTClaims{
		ID:         id,
		EmailAddrs: email,
		UserName:   username,
		FullName:   fullName,
		Role:       role,
		Department: department,
		TitleUser:  title,
		EmployeeID: employeeID,
		NIK:        nik,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenTTL())),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

func VerifyToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func TokenTTL() time.Duration {
	if m, err := strconv.Atoi(os.Getenv("JWT_EXPIRES_MINUTES")); err == nil && m > 0 {
		return time.Duration(m) * time.Minute
	}
	return 8 * time.Hour
}
