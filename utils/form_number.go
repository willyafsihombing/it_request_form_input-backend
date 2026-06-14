package utils

import (
	"database/sql"
	"fmt"
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

func BuildFormNumber(seq int) string {
	now := time.Now()
	return fmt.Sprintf("%d-ITR-Unggul-%02d%d", seq, int(now.Month()), now.Year())
}
