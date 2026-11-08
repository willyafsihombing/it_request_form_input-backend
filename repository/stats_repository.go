package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type StatsRepository struct {
	DB *sql.DB
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{DB: db}
}

type Stats struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	InReview  int `json:"inReview"`
	Approved  int `json:"approved"`
	Rejected  int `json:"rejected"`
	ThisMonth int `json:"thisMonth"`
}

func (r *StatsRepository) FetchStats(reqName, createdBy string) (*Stats, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	conditions := []string{"deleted_at IS NULL"}
	args := []interface{}{monthStart} // $1
	argIdx := 2

	if createdBy != "" {
		conditions = append(conditions, fmt.Sprintf("created_by_user_id = $%d", argIdx))
		args = append(args, createdBy)
		argIdx++
	}

	if reqName != "" {
		conditions = append(conditions, fmt.Sprintf("UPPER(TRIM(req_name)) = UPPER(TRIM($%d))", argIdx))
		args = append(args, reqName)
		argIdx++
	}

	query := `
		SELECT
			COUNT(*)                                       AS total,
			COUNT(*) FILTER (WHERE status = 'pending')     AS pending,
			COUNT(*) FILTER (WHERE status = 'in_review')   AS in_review,
			COUNT(*) FILTER (WHERE status = 'approved')    AS approved,
			COUNT(*) FILTER (WHERE status = 'rejected')    AS rejected,
			COUNT(*) FILTER (WHERE created_at >= $1)       AS this_month
		FROM requests
		WHERE ` + strings.Join(conditions, " AND ")

	var stats Stats
	err := r.DB.QueryRow(query, args...).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.InReview,
		&stats.Approved,
		&stats.Rejected,
		&stats.ThisMonth,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
