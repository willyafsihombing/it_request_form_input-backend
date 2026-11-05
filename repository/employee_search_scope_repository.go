package repository

import (
	"it-request-backend/models"
	"strings"
)

func (r *EmployeeRepository) SearchScoped(q, departemen string, limit int) ([]models.EmployeeOption, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	q = escapeLike(strings.TrimSpace(q))
	departemen = strings.TrimSpace(departemen)

	rows, err := r.db.Query(`
		SELECT id, nik, nama,
		       COALESCE(jabatan,''), COALESCE(departemen,''), COALESCE(level,''), staff_type
		FROM employees
		WHERE deleted_at IS NULL AND is_active
		  AND ($1::text = '' OR nama ILIKE '%' || $1 || '%' OR nik ILIKE $1 || '%')
		  AND ($2::text = '' OR lower(departemen) = lower($2))
		ORDER BY nama	
		LIMIT $3`, q, departemen, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.EmployeeOption{}
	for rows.Next() {
		var o models.EmployeeOption
		if err := rows.Scan(&o.ID, &o.NIK, &o.Nama, &o.Jabatan, &o.Departemen, &o.Level, &o.StaffType); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
