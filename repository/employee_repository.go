package repository

import (
	"database/sql"
	"errors"
	"it-request-backend/models"
	"strings"

	"github.com/lib/pq"
)

var ErrEmployeeNIKExists = errors.New("NIK sudah terdaftar(bisa jadi karyawan sudah didaftarkan sebelumnya)")

type EmployeeRepository struct {
	db *sql.DB
}

func NewEmployeeRepository(db *sql.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

const employeeColumns = `
	id, nik, nama, COALESCE(jabatan, ''), COALESCE(departemen, ''), COALESCE(level, ''), staff_type, is_active, deleted_at, created_at, updated_at
`

type rowscanner interface {
	Scan(dest ...any) error
}

func scanEmployee(s rowscanner) (*models.Employee, error) {
	var e models.Employee
	err := s.Scan(
		&e.ID,
		&e.NIK,
		&e.Nama,
		&e.Jabatan,
		&e.Departemen,
		&e.Level,
		&e.StaffType,
		&e.IsActive,
		&e.DeletedAt,
		&e.CreatedAt,
		&e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func (r *EmployeeRepository) FindByID(id string) (*models.Employee, error) {
	row := r.db.QueryRow(
		`SELECT`+employeeColumns+` FROM employees WHERE id = $1 AND deleted_at IS NULL`, id)
	e, err := scanEmployee(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return e, err
}

func (r *EmployeeRepository) Search(q string, limit int) ([]models.EmployeeOption, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	q = escapeLike(strings.TrimSpace(q))

	rows, err := r.db.Query(`
		SELECT id, nik, nama,
		       COALESCE(jabatan,''), COALESCE(departemen,''), COALESCE(level,''), staff_type
		FROM employees
		WHERE deleted_at IS NULL AND is_active
		  AND ($1::text = '' OR nama ILIKE '%' || $1 || '%' OR nik ILIKE $1 || '%')
		ORDER BY nama
		LIMIT $2`, q, limit)
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

// List — untuk halaman kelola karyawan (termasuk yang nonaktif), dengan pagination
func (r *EmployeeRepository) List(q, departemen, status, staffType, account string, page, perPage int) ([]models.Employee, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}
	q = escapeLike(strings.TrimSpace(q))
	departemen = strings.TrimSpace(departemen)
	if status != "active" && status != "inactive" {
		status = ""
	}
	if staffType != models.StaffTypeStaff && staffType != models.StaffTypeNonStaff {
		staffType = ""
	}
	if account != "has" && account != "no" {
		account = ""
	}

	where := `deleted_at IS NULL
		AND ($1::text = '' OR nama ILIKE '%' || $1 || '%' OR nik ILIKE $1 || '%')
		AND ($2::text = '' OR departemen = $2)
		AND ($3::text = '' OR ($3 = 'active' AND is_active) OR($3 = 'inactive' AND NOT is_active)) 
		AND ($4::text = '' OR staff_type = $4)
		AND ($5::text = '' 
			OR ($5 = 'has' AND EXISTS(SELECT 1 FROM users u WHERE u.employee_id = employees.id)) 
			OR ($5 = 'no' AND NOT EXISTS(SELECT 1 FROM users u WHERE u.employee_id = employees.id)))`

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM employees WHERE `+where, q, departemen, status, staffType, account).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		`SELECT `+employeeColumns+` FROM employees WHERE `+where+
			` ORDER BY nama LIMIT $6 OFFSET $7`,
		q, departemen, status, staffType, account, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []models.Employee{}
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *e)
	}
	return list, total, rows.Err()
}

func (r *EmployeeRepository) Create(req models.EmployeeRequest) (*models.Employee, error) {
	row := r.db.QueryRow(`
		INSERT INTO employees (nik, nama, jabatan, departemen, level, staff_type, is_active)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6, COALESCE($7::boolean, true))
		RETURNING `+employeeColumns,
		strings.TrimSpace(req.NIK), strings.TrimSpace(req.Nama),
		req.Jabatan, req.Departemen, req.Level, req.StaffType, req.IsActive)

	e, err := scanEmployee(row)
	if isUniqueViolation(err) {
		return nil, ErrEmployeeNIKExists
	}
	return e, err
}

func (r *EmployeeRepository) Update(id string, req models.EmployeeRequest) (*models.Employee, error) {
	row := r.db.QueryRow(`
		UPDATE employees SET
			nik        = $2,
			nama       = $3,
			jabatan    = NULLIF($4,''),
			departemen = NULLIF($5,''),
			level      = NULLIF($6,''),
			staff_type = $7,
			is_active  = COALESCE($8::boolean, is_active),
			updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+employeeColumns,
		id, strings.TrimSpace(req.NIK), strings.TrimSpace(req.Nama),
		req.Jabatan, req.Departemen, req.Level, req.StaffType, req.IsActive)

	e, err := scanEmployee(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if isUniqueViolation(err) {
		return nil, ErrEmployeeNIKExists
	}
	return e, err
}

func (r *EmployeeRepository) SoftDelete(id string) (bool, error) {
	res, err := r.db.Exec(`
		UPDATE employees
		SET deleted_at = now(), is_active = false, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *EmployeeRepository) UpsertMany(reqs []models.EmployeeRequest) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
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
			updated_at = now()`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, q := range reqs {
		if _, err := stmt.Exec(
			strings.TrimSpace(q.NIK), strings.TrimSpace(q.Nama),
			q.Jabatan, q.Departemen, q.Level, q.StaffType, q.IsActive,
		); err != nil {
			return 0, err
		}
		count++
	}
	return count, tx.Commit()
}

func (r *EmployeeRepository) Stats() (*models.EmployeeStats, error) {
	var s models.EmployeeStats
	err := r.db.QueryRow(
		`SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE is_active),	
			COUNT(*) FILTER (WHERE NOT is_active),	
			COUNT(*) FILTER (WHERE staff_type = 'staff'),	
			COUNT(*) FILTER (WHERE staff_type = 'non_staff')
		FROM employees
		WHERE deleted_at IS NULL`,
	).Scan(&s.Total, &s.Active, &s.Inactive, &s.Staff, &s.NonStaff)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(`
		SELECT DISTINCT departemen
		FROM employees
		WHERE deleted_at IS NULL AND departemen IS NOT NULL AND departemen <> ''
		ORDER BY departemen`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	s.Departments = []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		s.Departments = append(s.Departments, d)
	}
	return &s, rows.Err()
}
