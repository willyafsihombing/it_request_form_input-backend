package utils

import (
	"os"
	"strings"
)

func createBlockedLevels() map[string]bool {
	raw := os.Getenv("ITR_CREATE_BLOCKED_LEVELS")
	if strings.TrimSpace(raw) == "" {
		raw = "manager"
	}
	set := map[string]bool{}
	for _, v := range strings.Split(raw, ",") {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			set[v] = true
		}
	}
	return set
}

func IsCreateBlockedLevel(level string) bool {
	return createBlockedLevels()[strings.ToLower(strings.TrimSpace(level))]
}
