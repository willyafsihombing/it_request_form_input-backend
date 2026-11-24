package dashboard

import (
	"database/sql"
	"it-request-backend/internal/modules/itrequest"
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

func (r *StatsRepository) FetchStats(scope itrequest.ViewScope) (*Stats, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	conditions := []string{"deleted_at IS NULL"}
	args := []interface{}{monthStart} // $1

	if cl, a := scope.Clause(2); cl != "" {
		conditions = append(conditions, cl)
		args = append(args, a...)
	}

	query := `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'pending'
			                   AND COALESCE(approval_status, '') <> 'in_review')        AS pending,
			COUNT(*) FILTER (WHERE status = 'in_review'
			                   OR  approval_status = 'in_review')                       AS in_review,
			COUNT(*) FILTER (WHERE status = 'approved')                                 AS approved,
			COUNT(*) FILTER (WHERE status = 'rejected')                                 AS rejected,
			COUNT(*) FILTER (WHERE created_at >= $1)                                    AS this_month
		FROM requests
		WHERE ` + strings.Join(conditions, " AND ")

	var stats Stats
	err := r.DB.QueryRow(query, args...).Scan(
		&stats.Total, &stats.Pending, &stats.InReview,
		&stats.Approved, &stats.Rejected, &stats.ThisMonth,
	)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}
