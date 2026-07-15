package utils

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

func GenerateFormNumber(db *sql.DB) (int, error) {
	var seq int

	err := db.QueryRow("SELECT nextval('form_number_seq')").Scan(&seq)
	if err != nil {
		return 0, err
	}
	return seq, nil
}

// CREATE SEQUENCE IF NOT EXISTS form_number_seq START 175;

func sanitizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	reg := regexp.MustCompile(`[^a-zA-Z0-9\s]+`)
	title = reg.ReplaceAllString(title, "")

	title = strings.Join(strings.Fields(title), " ")

	return title
}
func BuildFormNumber(seq int, requestTitle string) string {
	now := time.Now()
	base := fmt.Sprintf("%d-ITR-Unggul-%02d%d", seq, int(now.Month()), now.Year())

	title := sanitizeTitle(requestTitle)
	if title == "" {
		return base
	}

	return fmt.Sprintf("%s-%s", base, strings.ToUpper(title))
}
