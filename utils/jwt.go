package utils

import (
	"os"
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
	jwt.RegisteredClaims
}

func GenerateToken(id, email, username, fullName, role, department, title string) (string, error) {
	claims := JWTClaims{
		ID:         id,
		EmailAddrs: email,
		UserName:   username,
		FullName:   fullName,
		Role:       role,
		Department: department,
		TitleUser:  title,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
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
