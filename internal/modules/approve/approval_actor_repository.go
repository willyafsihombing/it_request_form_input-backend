package approve

import (
	"context"
	"database/sql"
	"errors"
)

func (r *ActionRepository) EmployeeIDByUser(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(employee_id::text, '') FROM users WHERE id = $1`, userID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
