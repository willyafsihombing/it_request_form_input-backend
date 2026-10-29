package repository

import (
	"it-request-backend/models"
	"strings"

	"github.com/lib/pq"
)

func (r *EmployeeRepository) ExistingNIKs(niks []string) (map[string]bool, error) {
	out := make(map[string]bool, len(niks))
	if len(niks) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(`SELECT nik FROM employees WHERE nik = ANY($1)`, pq.Array(niks))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var nik string
		if err := rows.Scan(&nik); err != nil {
			return nil, err
		}
		out[nik] = true
	}
	return out, rows.Err()
}

func (r *EmployeeRepository) ImportMany(reqs []models.EmployeeRequest) (created, updated int, err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO employees (nik, nama, jabatan, departemen, level, staff_type, is_active)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6, COALESCE($7::boolean, true))
		ON CONFLICT (nik) DO UPDATE SET
			nama       = EXCLUDED.nama,
			jabatan    = EXCLUDED.jabatan,
			departemen = EXCLUDED.departemen,
			level      = EXCLUDED.level,
			staff_type = EXCLUDED.staff_type,
			is_active  = EXCLUDED.is_active,
			deleted_at = NULL,
			updated_at = now()
		RETURNING (xmax = 0)`)
	if err != nil {
		return 0, 0, err
	}
	defer stmt.Close()

	for _, q := range reqs {
		var inserted bool
		if err := stmt.QueryRow(
			strings.TrimSpace(q.NIK), strings.TrimSpace(q.Nama),
			q.Jabatan, q.Departemen, q.Level, q.StaffType, q.IsActive,
		).Scan(&inserted); err != nil {
			return 0, 0, err
		}
		if inserted {
			created++
		} else {
			updated++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return created, updated, nil
}
