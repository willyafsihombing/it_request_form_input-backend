package repository

import (
	"database/sql"
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

func (r *StatsRepository) FetchStats(reqName string) (*Stats, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var stats Stats
	var err error

	if reqName == "" {
		err = r.DB.QueryRow(`
			SELECT
				COUNT(*)										AS total,
				COUNT(*) FILTER (WHERE status = 'pending') 		AS pending,	
				COUNT(*) FILTER (WHERE status = 'in_review')	AS in_review,
				COUNT(*) FILTER (WHERE status = 'approved')		AS approved,
				COUNT(*) FILTER (WHERE status = 'rejected')		AS rejected,
				COUNT(*) FILTER (WHERE created_at >= $1)		AS this_month
					FROM requests
					WHERE deleted_at IS NULL`, monthStart).Scan(
			&stats.Total,
			&stats.Pending,
			&stats.InReview,
			&stats.Approved,
			&stats.Rejected,
			&stats.ThisMonth,
		)
	} else {
		err = r.DB.QueryRow(`
            SELECT
                COUNT(*)                                                AS total,
                COUNT(*) FILTER (WHERE status = 'pending')             AS pending,
                COUNT(*) FILTER (WHERE status = 'in_review')           AS in_review,
                COUNT(*) FILTER (WHERE status = 'approved')            AS approved,
                COUNT(*) FILTER (WHERE status = 'rejected')            AS rejected,
                COUNT(*) FILTER (WHERE created_at >= $1)               AS this_month
            FROM requests
            WHERE deleted_at IS NULL
            AND req_name ILIKE $2
        `, monthStart, "%"+reqName+"%").Scan(
			&stats.Total,
			&stats.Pending,
			&stats.InReview,
			&stats.Approved,
			&stats.Rejected,
			&stats.ThisMonth,
		)
	}

	if err != nil {
		return nil, err
	}

	return &stats, err
}
