package utils

import (
	"github.com/gin-gonic/gin"
)

func CurrentClaims(c *gin.Context) *JWTClaims {
	v, ok := c.Get("user")
	if !ok {
		// fmt.Println("[currentClaims] key \"user\" tidak ada di context")
		return nil
	}
	switch t := v.(type) {
	case *JWTClaims:
		return t
	case JWTClaims:
		return &t
	default:
		// fmt.Printf("[currentClaims] tipe tidak dikenal: %T\n", v)
		return nil
	}
}
