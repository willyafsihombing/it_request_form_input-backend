package utils

import (
	"os"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func EmailDomain() string {
	if v := strings.TrimSpace(os.Getenv("EMAIL_DOMAIN")); v != "" {
		return strings.TrimPrefix(v, "@")
	}
	return "ungguldinamika.co.id"
}

func EmailLocalPart(fullName string) string {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(fullName)) {
		if strings.ContainsAny(w, ".,") {
			continue
		}
		w = nonAlnum.ReplaceAllString(w, "")
		if w != "" {
			words = append(words, w)
		}
	}
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	default:
		return words[0] + "." + words[len(words)-1]
	}
}
