package approve

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
)

var (
	ErrRouteNotFound    = errors.New("penunjukan tidak ditemuka")
	ErrRouteExists      = errors.New("pejabat ini sudah ditunjuk langkah tersebut")
	ErrEmployeeNotFound = errors.New("karyawan tidak ditemukan")
)

type RouteRepository struct {
	db *sql.DB
}

func NewRouteRepository(db *sql.DB) *RouteRepository {
	return &RouteRepository{db: db}
}

const routeSelect = `
	SELECT r.id, r.code, r.departemen, r.approver_employee_id,
		COALESCE(e.nama, ''), COALESCE(e.jabatan, ''),
		(e.deleted_at is NULL AND e.is_active
		AND EXISTS (SELECT 1 FROM users u WHERE u.employee_id::text = e.id::text)) AS active,
		r.created_at, r.updated_at
	FROM approval_routes r
	JOIN employees e ON e.id = r.approver_employee_id`

func scanRoute(sc interface{ Scan(dest ...any) error }) (Route, error) {
	var r Route
	var code string
	err := sc.Scan(&r.ID, &code, &r.Departemen, &r.ApproverEmployeeID,
		&r.ApproverNama, &r.ApproverJabatan, &r.ApproverActive,
		&r.CreatedAt, &r.UpdatedAt)
	r.Code = StepCode(code)
	r.Label = r.Code.Label()
	return r, err
}

func (r *RouteRepository) query(ctx context.Context, where string, args ...any) ([]Route, error) {
	rows, err := r.db.QueryContext(ctx, routeSelect+where+" ORDER BY r.departemen, r.created_at", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Route{}
	for rows.Next() {
		rt, err := scanRoute(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Code.Order() != out[j].Code.Order() {
			return out[i].Code.Order() < out[j].Code.Order()
		}
		return out[i].Departemen < out[j].Departemen
	})
	return out, nil
}

func (r *RouteRepository) List(ctx context.Context, code StepCode, departemen string) ([]Route, error) {
	return r.query(ctx,
		" WHERE ($1 = '' OR r.code = $1) AND ($2 = '' OR r.departemen = $2)",
		string(code), departemen)
}

func (r *RouteRepository) ListProblems(ctx context.Context) ([]Route, error) {
	return r.query(ctx, ` WHERE NOT (e.deleted_at IS NULL AND e.is_active
        AND EXISTS (SELECT 1 FROM users u WHERE u.employee_id::text = e.id::text))`)
}

func (r *RouteRepository) GetByID(ctx context.Context, id string) (Route, error) {
	rt, err := scanRoute(r.db.QueryRowContext(ctx, routeSelect+" WHERE r.id::text = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Route{}, ErrRouteNotFound
	}
	return rt, err
}

func (r *RouteRepository) Create(ctx context.Context, in RouteInput) (Route, error) {
	if msg := in.Validate(); msg != "" {
		return Route{}, errors.New(msg)
	}

	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO approval_routes (code, departemen, approver_employee_id)
		SELECT $1, $2, e.id FROM employees e
		WHERE e.id::text = $3 AND e.deleted_at IS NULL
		RETURNING id`,
		string(in.Code), in.Departemen, in.ApproverEmployeeID).Scan(&id)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Route{}, ErrEmployeeNotFound
	case err != nil && (strings.Contains(err.Error(), "approval_routes_uniq") ||
		strings.Contains(err.Error(), "23505")):
		return Route{}, ErrRouteExists
	case err != nil:
		return Route{}, err
	}
	return r.GetByID(ctx, id)
}
func (r *RouteRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM approval_routes WHERE id::text = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrRouteNotFound
	}
	return nil
}
