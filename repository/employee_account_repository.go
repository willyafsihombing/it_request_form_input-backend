package repository

import (
	"it-request-backend/models"
	"strings"

	"github.com/lib/pq"
)

func (r *EmployeeRepository) FindActiveByIDs(ids []string) ([]models.Employee, error) {
	out := []models.Employee{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(
		`SELECT `+employeeColumns+` FROM employees
		 WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL AND is_active`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *EmployeeRepository) IDsWithoutAccount(departemen string, limit int) ([]string, error) {
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	rows, err := r.db.Query(`
		SELECT e.id
		FROM employees e
		WHERE e.deleted_at IS NULL AND e.is_active
		  AND ($1::text = '' OR lower(e.departemen) = lower($1))
		  AND NOT EXISTS (
		      SELECT 1 FROM users u
		      WHERE u.employee_id = e.id AND u.deleted_at IS NULL)
		ORDER BY e.nama
		LIMIT $2`, strings.TrimSpace(departemen), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
