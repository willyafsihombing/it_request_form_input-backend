package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func printMAC(id string, exp int64) string {
	m := hmac.New(sha256.New, []byte(os.Getenv("JWT_SECRET")))
	m.Write([]byte(fmt.Sprintf("print:%s:%d", id, exp)))
	return hex.EncodeToString(m.Sum(nil))
}

func PrintToken(id string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	return fmt.Sprintf("%d.%s", exp, printMAC(id, exp))
}

func VerifyPrintToken(id, tok string) bool {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(parts[1]), []byte(printMAC(id, exp)))
}
