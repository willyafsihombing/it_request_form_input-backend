package utils

import (
	"errors"
	"os"
)

func CheckJWTSecret() error {
	if len(os.Getenv("JWT_SECRET")) < 32 {
		return errors.New("JWT_SECRET belum diisi atau kurang dari 32 karakter")
	}
	return nil
}
