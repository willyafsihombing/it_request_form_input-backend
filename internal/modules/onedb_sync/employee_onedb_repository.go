package onedbsync

import (
	"context"
	"database/sql"
	"fmt"
	"uuid"
)

type EmployeeSyncRepository struct {
	DB *sql.DB
}

func NewEmployeeSyncRepository(db *sql.DB) *EmployeeSyncRepository {
	return &EmployeeSyncRepository{DB: db}
}

const upsertSQL = `
	INSERT INTO employees (id, nik, nama, jabatan, departemen, level, staff_type,
		is_active, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	ON CONFLICT (nik) DO UPDATE SET
		nama       = EXCLUDED.nama,
		jabatan    = EXCLUDED.jabatan,
		departemen = EXCLUDED.departemen,
		level      = EXCLUDED.level,
		staff_type = EXCLUDED.staff_type,
		is_active  = EXCLUDED.is_active,
		deleted_at = NULL,
		updated_at = NOW()
	RETURNING (xmax = 0)
`

func (r *EmployeeSyncRepository) SyncUpsert(ctx context.Context, rows []EmployeeSync) (inserted, updated int, err error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	for _, e := range rows {
		var isNew bool
		err := tx.QueryRowContext(ctx, upsertSQL,
			uuid.New().String(), e.NIK, e.Nama, e.Jabatan, e.Departemen, e.Level, e.StaffType, e.IsActive,
		).Scan(&isNew)
		if err != nil {
			return 0, 0, fmt.Errorf("upsert NIK: %s: %w", e.NIK, err)
		}
		if isNew {
			inserted++
		} else {
			updated++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return inserted, updated, nil
}
