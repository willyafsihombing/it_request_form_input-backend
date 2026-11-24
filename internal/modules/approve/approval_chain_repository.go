package approve

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type PlannedStep struct {
	Code      StepCode
	Label     string
	StepOrder int
	Status    StepStatus
	Approvers []StepApprover
	Carried   bool
}

type Plan struct {
	Departemen string
	Steps      []PlannedStep
	Problems   []string
}

func (p Plan) OK() bool { return len(p.Problems) == 0 }

func (p Plan) ActiveOrder() int {
	for _, s := range p.Steps {
		if s.Status == StepActive {
			return s.StepOrder
		}
	}
	return 0
}

type ChainError struct {
	Problems []string
}

func (e *ChainError) Error() string {
	return "rantai persetujuan belum bisa dibuat: " + strings.Join(e.Problems, "; ")
}

type routeRow struct {
	Code       StepCode
	Departemen string
	EmployeeID string
	Nama       string
	Jabatan    string
	Usable     bool
}

func buildPlan(rows []routeRow, departemen, requesterEmployeeID string) Plan {
	return buildPlanFrom(rows, departemen, requesterEmployeeID, 1)
}

func buildPlanFrom(rows []routeRow, departemen, requesterEmployeeID string, fromOrder int) Plan {
	plan := Plan{Departemen: departemen}

	if strings.TrimSpace(departemen) == "" {
		plan.Problems = append(plan.Problems, "departemen requester kosong")
		return plan
	}

	byCode := map[StepCode][]routeRow{}
	for _, r := range rows {
		if !r.Code.Valid() {
			continue
		}
		if r.Code.IsDepartmentScoped() {
			if r.Departemen != departemen {
				continue
			}
		} else if r.Departemen != "" {
			continue
		}
		byCode[r.Code] = append(byCode[r.Code], r)
	}

	for _, code := range AllSteps {
		rs := byCode[code]
		step := PlannedStep{Code: code, Label: code.Label(), StepOrder: code.Order()}
		if step.StepOrder < fromOrder {
			step.Carried = true
			plan.Steps = append(plan.Steps, step)
			continue
		}

		var usable []StepApprover
		hadRequester := false
		for _, r := range rs {
			if !r.Usable {
				continue
			}
			if requesterEmployeeID != "" && r.EmployeeID == requesterEmployeeID {
				hadRequester = true
				continue
			}
			usable = append(usable, StepApprover{EmployeeID: r.EmployeeID, Nama: r.Nama, Jabatan: r.Jabatan})
		}

		switch {
		case len(rs) == 0 && code == StepSrManager:
			step.Status = StepSkipped
		case len(rs) == 0:
			plan.Problems = append(plan.Problems, missingRoute(code, departemen))
		case len(usable) == 0 && hadRequester:
			step.Status = StepSkipped
		case len(usable) == 0:
			plan.Problems = append(plan.Problems,
				fmt.Sprintf("semua pejabat %s nonaktif atau belum punya akun", code.Label()))
		default:
			step.Approvers = usable
		}
		plan.Steps = append(plan.Steps, step)
	}

	activated := false
	for i := range plan.Steps {
		if plan.Steps[i].Carried || plan.Steps[i].Status != "" {
			continue
		}
		if !activated {
			plan.Steps[i].Status = StepActive
			activated = true
		} else {
			plan.Steps[i].Status = StepWaiting
		}
	}

	if !activated && len(plan.Problems) == 0 {
		plan.Problems = append(plan.Problems, "tidak ada langkah yang perlu ditandatangani")
	}
	return plan
}

func missingRoute(code StepCode, departemen string) string {
	if code.IsDepartmentScoped() {
		return fmt.Sprintf("rute %s untuk departemen %s belum diatur", code.Label(), departemen)
	}
	return fmt.Sprintf("rute %s belum diatur", code.Label())
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type ChainRepository struct {
	db *sql.DB
}

func NewChainRepository(db *sql.DB) *ChainRepository {
	return &ChainRepository{db: db}
}

const routesForDeptSQL = `
SELECT r.code,
       COALESCE(r.departemen, ''),
       r.approver_employee_id::text,
       COALESCE(e.nama, ''),
       COALESCE(e.jabatan, ''),
       (COALESCE(e.is_active, false)
        AND e.deleted_at IS NULL
        AND EXISTS (SELECT 1 FROM users u WHERE u.employee_id = e.id)) AS usable
FROM approval_routes r
LEFT JOIN employees e ON e.id = r.approver_employee_id
WHERE COALESCE(r.departemen, '') IN ('', $1)
ORDER BY r.code, e.nama`

func resolve(ctx context.Context, q querier, departemen, requesterEmployeeID string, fromOrder int) (Plan, error) {
	rows, err := q.QueryContext(ctx, routesForDeptSQL, departemen)
	if err != nil {
		return Plan{}, fmt.Errorf("baca rute: %w", err)
	}
	defer rows.Close()

	var list []routeRow
	for rows.Next() {
		var r routeRow
		var code string
		if err := rows.Scan(&code, &r.Departemen, &r.EmployeeID, &r.Nama, &r.Jabatan, &r.Usable); err != nil {
			return Plan{}, fmt.Errorf("scan rute: %w", err)
		}
		r.Code = StepCode(code)
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return Plan{}, err
	}
	return buildPlanFrom(list, departemen, requesterEmployeeID, fromOrder), nil
}

func (r *ChainRepository) ResolveChain(ctx context.Context, departemen, requesterEmployeeID string) (Plan, error) {
	return resolve(ctx, r.db, departemen, requesterEmployeeID, 1)
}

func (r *ChainRepository) IsDesignated(ctx context.Context, employeeID string, code StepCode) (bool, error) {
	if employeeID == "" {
		return false, nil
	}
	var ok bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM approval_routes
			WHERE approver_employee_id::text = $1 AND code = $2
		)`, employeeID, string(code)).Scan(&ok)
	return ok, err
}

var ErrNotRejected = errors.New("request tidak sedang ditolak, tidak bisa diajukan ulang")

const (
	sqlCarryStep = `
INSERT INTO approval_steps (id, request_id, round, code, step_order, status,
    acted_by_employee_id, acted_by_user_id, acted_by_nama, acted_by_jabatan,
    acted_at, note, signature_image, carried_over, created_at)
SELECT $1::uuid, request_id, $2::int, code, step_order, status,
    acted_by_employee_id, acted_by_user_id, acted_by_nama, acted_by_jabatan,
    acted_at, note, signature_image, true, NOW()
FROM approval_steps
WHERE request_id = $3 AND round = $4::int AND step_order = $5::int`

	sqlCarryApprovers = `
INSERT INTO approval_step_approvers (step_id, employee_id, nama, jabatan)
SELECT $1::uuid, a.employee_id, a.nama, a.jabatan
FROM approval_step_approvers a
JOIN approval_steps o ON o.id = a.step_id
WHERE o.request_id = $2 AND o.round = $3::int AND o.step_order = $4::int`

	sqlLatestRejected = `
SELECT round, step_order FROM approval_steps
WHERE request_id = $1 AND status = 'rejected'
ORDER BY round DESC LIMIT 1`
)

func persistPlan(ctx context.Context, tx *sql.Tx, requestID string, plan Plan, round, fromRound int) error {
	for _, s := range plan.Steps {
		stepID := uuid.NewString()

		if s.Carried {
			res, err := tx.ExecContext(ctx, sqlCarryStep, stepID, round, requestID, fromRound, s.StepOrder)
			if err != nil {
				return fmt.Errorf("bawa langkah %s: %w", s.Code, err)
			}
			if n, _ := res.RowsAffected(); n != 1 {
				return fmt.Errorf("langkah %s putaran %d tidak ditemukan", s.Code, fromRound)
			}
			if _, err := tx.ExecContext(ctx, sqlCarryApprovers, stepID, requestID, fromRound, s.StepOrder); err != nil {
				return fmt.Errorf("bawa pejabat langkah %s: %w", s.Code, err)
			}
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO approval_steps (id, request_id, round, code, step_order, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
			stepID, requestID, round, string(s.Code), s.StepOrder, string(s.Status),
		); err != nil {
			return fmt.Errorf("simpan langkah %s: %w", s.Code, err)
		}
		for _, a := range s.Approvers {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO approval_step_approvers (step_id, employee_id, nama, jabatan)
				VALUES ($1, $2, $3, $4)`,
				stepID, a.EmployeeID, a.Nama, a.Jabatan,
			); err != nil {
				return fmt.Errorf("simpan pejabat langkah %s: %w", s.Code, err)
			}
		}
	}
	return nil
}

func markInReview(ctx context.Context, tx *sql.Tx, requestID string, currentStep, round int) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE requests
		SET approval_status = $2, current_step = $3, approval_round = $4
		WHERE id = $1`,
		requestID, string(RequestInReview), currentStep, round,
	)
	if err != nil {
		return fmt.Errorf("update status request: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("request %s tidak ditemukan", requestID)
	}
	return nil
}

func (r *ChainRepository) InsertChain(
	ctx context.Context, tx *sql.Tx,
	requestID, departemen, requesterEmployeeID string, round int,
) (Plan, error) {
	plan, err := resolve(ctx, tx, departemen, requesterEmployeeID, 1)
	if err != nil {
		return plan, err
	}
	if !plan.OK() {
		return plan, &ChainError{Problems: plan.Problems}
	}
	if err := persistPlan(ctx, tx, requestID, plan, round, 0); err != nil {
		return plan, err
	}
	return plan, markInReview(ctx, tx, requestID, plan.ActiveOrder(), round)
}

func (r *ChainRepository) ResubmitChain(
	ctx context.Context, tx *sql.Tx,
	requestID, departemen, requesterEmployeeID string, restart bool,
) (Plan, int, error) {
	status, err := r.LockRequestStatus(ctx, tx, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, 0, ErrRequestNotFound
	}
	if err != nil {
		return Plan{}, 0, err
	}
	if status != RequestRejected {
		return Plan{}, 0, ErrNotRejected
	}

	fromOrder, fromRound := 1, 0
	var rejRound, rejOrder int
	err = tx.QueryRowContext(ctx, sqlLatestRejected, requestID).Scan(&rejRound, &rejOrder)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// tidak ada catatan penolakan: mulai dari awal
	case err != nil:
		return Plan{}, 0, fmt.Errorf("cari langkah yang menolak: %w", err)
	default:
		fromRound = rejRound
		if !restart {
			fromOrder = rejOrder
		}
	}

	round, err := r.NextRound(ctx, tx, requestID)
	if err != nil {
		return Plan{}, 0, err
	}
	plan, err := resolve(ctx, tx, departemen, requesterEmployeeID, fromOrder)
	if err != nil {
		return plan, round, err
	}
	if !plan.OK() {
		return plan, round, &ChainError{Problems: plan.Problems}
	}
	if err := persistPlan(ctx, tx, requestID, plan, round, fromRound); err != nil {
		return plan, round, err
	}
	return plan, round, markInReview(ctx, tx, requestID, plan.ActiveOrder(), round)
}

func (r *ChainRepository) LockRequestStatus(ctx context.Context, tx *sql.Tx, requestID string) (RequestStatus, error) {
	var st string
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(approval_status, '') FROM requests WHERE id = $1 FOR UPDATE`, requestID,
	).Scan(&st)
	if err != nil {
		return "", err
	}
	return RequestStatus(st), nil
}

func (r *ChainRepository) NextRound(ctx context.Context, tx *sql.Tx, requestID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(round), 0) + 1 FROM approval_steps WHERE request_id = $1`, requestID,
	).Scan(&n)
	return n, err
}
