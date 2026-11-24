package approve

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrRequestNotFound = errors.New("request tidak ditemukan")
	ErrNotInReview     = errors.New("request tidak sedang dalam proses persetujuan")
	ErrNoActiveStep    = errors.New("tidak ada langkah persetujuan yang aktif")
	ErrNotApprover     = errors.New("anda tidak berhak menindak langkah ini")
	ErrNoteRequired    = errors.New("alasan penolakan wajib diisi")
)

type Actor struct {
	UserID     string
	EmployeeID string
}

type SignatureColumn struct {
	Name string
	At   string
}

type SignatureMap map[StepCode]SignatureColumn

var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func (m SignatureMap) validate() error {
	for code, c := range m {
		if !code.Valid() {
			return fmt.Errorf("kode langkah %q tidak dikenal", code)
		}
		if !identRe.MatchString(c.Name) {
			return fmt.Errorf("nama kolom tanda tangan %s tidak valid: %q", code, c.Name)
		}
		if c.At != "" && !identRe.MatchString(c.At) {
			return fmt.Errorf("nama kolom waktu tanda tangan %s tidak valid: %q", code, c.At)
		}
	}
	return nil
}

func signSQL(c SignatureColumn) string {
	if c.At != "" {
		return fmt.Sprintf(`UPDATE requests SET "%s" = $2, "%s" = NOW() WHERE id = $1`, c.Name, c.At)
	}
	return fmt.Sprintf(`UPDATE requests SET "%s" = $2 WHERE id = $1`, c.Name)
}

type ActionResult struct {
	Code          StepCode      `json:"code"`
	Decision      StepStatus    `json:"decision"`
	RequestStatus RequestStatus `json:"requestStatus"`
	CurrentStep   int           `json:"currentStep"`
	NextCode      StepCode      `json:"nextCode,omitempty"`
}

type InboxItem struct {
	RequestID   string    `json:"requestId"`
	Code        StepCode  `json:"code"`
	Label       string    `json:"label"`
	StepOrder   int       `json:"stepOrder"`
	Round       int       `json:"round"`
	SubmittedAt time.Time `json:"submittedAt"`
}

const (
	sqlLockRequest = `
SELECT COALESCE(approval_status, ''), approval_round
FROM requests WHERE id = $1 FOR UPDATE`

	sqlLockActiveStep = `
SELECT id, code, step_order
FROM approval_steps
WHERE request_id = $1 AND round = $2 AND status = 'active'
FOR UPDATE`

	sqlActorOnStep = `
SELECT e.nama, e.jabatan
FROM approval_step_approvers a
JOIN employees e ON e.id = a.employee_id
WHERE a.step_id = $1 AND a.employee_id = $2
  AND e.is_active AND e.deleted_at IS NULL`

	sqlMarkStep = `
UPDATE approval_steps
SET status = $2,
    acted_by_employee_id = $3,
    acted_by_user_id = NULLIF($4::text, ''),
    acted_by_nama = $5,
    acted_by_jabatan = $6,
    acted_at = NOW(),
    note = $7
WHERE id = $1 AND status = 'active'`

	sqlNextWaiting = `
SELECT id, code, step_order
FROM approval_steps
WHERE request_id = $1 AND round = $2 AND status = 'waiting'
ORDER BY step_order
LIMIT 1
FOR UPDATE`

	sqlActivateStep = `UPDATE approval_steps SET status = 'active' WHERE id = $1 AND status = 'waiting'`

	sqlSetCurrentStep = `UPDATE requests SET current_step = $2 WHERE id = $1`

	// status = 'approved' / 'rejected' menyelaraskan kolom status lama (dipakai Resubmit dan aturan edit)
	sqlRequestApproved = `UPDATE requests SET approval_status = 'approved', current_step = 0, status = 'approved', updated_at = NOW() WHERE id = $1`

	sqlRequestRejected = `UPDATE requests SET approval_status = 'rejected', current_step = 0, status = 'rejected', updated_at = NOW() WHERE id = $1`

	sqlInbox = `
SELECT s.request_id, s.code, s.step_order, s.round, s.created_at, COUNT(*) OVER()
FROM approval_steps s
JOIN approval_step_approvers a ON a.step_id = s.id AND a.employee_id = $1
JOIN requests q ON q.id = s.request_id
WHERE s.status = 'active'
  AND q.approval_status = 'in_review'
  AND s.round = q.approval_round
ORDER BY s.created_at, s.request_id
LIMIT $2 OFFSET $3`

	sqlPendingCount = `
SELECT COUNT(*)
FROM approval_steps s
JOIN approval_step_approvers a ON a.step_id = s.id AND a.employee_id = $1
JOIN requests q ON q.id = s.request_id
WHERE s.status = 'active'
  AND q.approval_status = 'in_review'
  AND s.round = q.approval_round`

	sqlProgressRequest = `
SELECT COALESCE(approval_status, ''), approval_round FROM requests WHERE id = $1`

	sqlProgressSteps = `
SELECT id, code, step_order, status,
       acted_by_employee_id::text, acted_by_user_id,
       acted_by_nama, acted_by_jabatan, acted_at, note, carried_over, created_at
FROM approval_steps
WHERE request_id = $1 AND round = $2
ORDER BY step_order`

	sqlProgressApprovers = `
SELECT a.step_id, a.employee_id::text, a.nama, a.jabatan
FROM approval_step_approvers a
JOIN approval_steps s ON s.id = a.step_id
WHERE s.request_id = $1 AND s.round = $2
ORDER BY a.nama`
)

type ActionRepository struct {
	db     *sql.DB
	sig    SignatureMap
	notify Notifier
}

func NewActionRepository(db *sql.DB, sig SignatureMap) (*ActionRepository, error) {
	if err := sig.validate(); err != nil {
		return nil, err
	}
	return &ActionRepository{db: db, sig: sig}, nil
}

type activeStep struct {
	ID    string
	Code  StepCode
	Order int
	Round int
}

type actorInfo struct{ Nama, Jabatan string }

func lockAndAuthorize(ctx context.Context, tx *sql.Tx, requestID string, actor Actor) (activeStep, actorInfo, error) {
	var st activeStep
	var ai actorInfo

	var status string
	err := tx.QueryRowContext(ctx, sqlLockRequest, requestID).Scan(&status, &st.Round)
	if errors.Is(err, sql.ErrNoRows) {
		return st, ai, ErrRequestNotFound
	}
	if err != nil {
		return st, ai, fmt.Errorf("kunci request: %w", err)
	}
	if RequestStatus(status) != RequestInReview {
		return st, ai, ErrNotInReview
	}

	var code string
	err = tx.QueryRowContext(ctx, sqlLockActiveStep, requestID, st.Round).Scan(&st.ID, &code, &st.Order)
	if errors.Is(err, sql.ErrNoRows) {
		return st, ai, ErrNoActiveStep
	}
	if err != nil {
		return st, ai, fmt.Errorf("kunci langkah aktif: %w", err)
	}
	st.Code = StepCode(code)

	if actor.EmployeeID == "" {
		return st, ai, ErrNotApprover
	}
	err = tx.QueryRowContext(ctx, sqlActorOnStep, st.ID, actor.EmployeeID).Scan(&ai.Nama, &ai.Jabatan)
	if errors.Is(err, sql.ErrNoRows) {
		return st, ai, ErrNotApprover
	}
	if err != nil {
		return st, ai, fmt.Errorf("cek penyetuju: %w", err)
	}
	return st, ai, nil
}

func markStep(ctx context.Context, tx *sql.Tx, st activeStep, to StepStatus, actor Actor, ai actorInfo, note string) error {
	res, err := tx.ExecContext(ctx, sqlMarkStep,
		st.ID, string(to), actor.EmployeeID, actor.UserID, ai.Nama, ai.Jabatan, note)
	if err != nil {
		return fmt.Errorf("simpan tindakan: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrNoActiveStep
	}
	return nil
}

func (r *ActionRepository) Approve(ctx context.Context, requestID string, actor Actor, note string) (ActionResult, error) {
	var out ActionResult

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	st, ai, err := lockAndAuthorize(ctx, tx, requestID, actor)
	if err != nil {
		return out, err
	}
	if err := r.CopySignatureToStep(ctx, tx, actor.UserID, st.ID); err != nil {
		return out, err
	}
	if err := markStep(ctx, tx, st, StepApproved, actor, ai, strings.TrimSpace(note)); err != nil {
		return out, err
	}

	if col, ok := r.sig[st.Code]; ok {
		if _, err := tx.ExecContext(ctx, signSQL(col), requestID, ai.Nama); err != nil {
			return out, fmt.Errorf("tulis tanda tangan %s: %w", st.Code, err)
		}
	}

	out.Code = st.Code
	out.Decision = StepApproved

	var nextID, nextCode string
	var nextOrder int
	err = tx.QueryRowContext(ctx, sqlNextWaiting, requestID, st.Round).Scan(&nextID, &nextCode, &nextOrder)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.ExecContext(ctx, sqlRequestApproved, requestID); err != nil {
			return out, fmt.Errorf("selesaikan request: %w", err)
		}
		out.RequestStatus = RequestApproved
		out.CurrentStep = 0
	case err != nil:
		return out, fmt.Errorf("cari langkah berikutnya: %w", err)
	default:
		if _, err := tx.ExecContext(ctx, sqlActivateStep, nextID); err != nil {
			return out, fmt.Errorf("aktifkan langkah berikutnya: %w", err)
		}
		if _, err := tx.ExecContext(ctx, sqlSetCurrentStep, requestID, nextOrder); err != nil {
			return out, fmt.Errorf("update langkah aktif: %w", err)
		}
		out.RequestStatus = RequestInReview
		out.CurrentStep = nextOrder
		out.NextCode = StepCode(nextCode)
	}

	if r.notify != nil {
		var nerr error
		if out.RequestStatus == RequestApproved {
			nerr = r.notify.RequestApproved(ctx, tx, requestID)
		} else {
			nerr = r.notify.StepActivated(ctx, tx, requestID)
		}
		if nerr != nil {
			return out, nerr
		}
	}

	if err := tx.Commit(); err != nil {
		return ActionResult{}, err
	}
	return out, nil
}

func (r *ActionRepository) Reject(ctx context.Context, requestID string, actor Actor, note string) (ActionResult, error) {
	var out ActionResult

	note = strings.TrimSpace(note)
	if note == "" {
		return out, ErrNoteRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	st, ai, err := lockAndAuthorize(ctx, tx, requestID, actor)
	if err != nil {
		return out, err
	}
	if err := markStep(ctx, tx, st, StepRejected, actor, ai, note); err != nil {
		return out, err
	}

	if r.notify != nil {
		if err := r.notify.RequestRejected(ctx, tx, requestID); err != nil {
			return out, err
		}
	}

	if _, err := tx.ExecContext(ctx, sqlRequestRejected, requestID); err != nil {
		return out, fmt.Errorf("tandai request ditolak: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{
		Code:          st.Code,
		Decision:      StepRejected,
		RequestStatus: RequestRejected,
		CurrentStep:   0,
	}, nil
}

func (r *ActionRepository) clearSQL() (string, bool) {
	var sets []string
	for _, code := range AllSteps {
		c, ok := r.sig[code]
		if !ok {
			continue
		}
		sets = append(sets, fmt.Sprintf(`"%s" = ''`, c.Name))
		if c.At != "" {
			sets = append(sets, fmt.Sprintf(`"%s" = NULL`, c.At))
		}
	}
	if len(sets) == 0 {
		return "", false
	}
	return "UPDATE requests SET " + strings.Join(sets, ", ") + " WHERE id = $1", true
}

func (r *ActionRepository) ClearSignatures(ctx context.Context, tx *sql.Tx, requestID string) error {
	q, ok := r.clearSQL()
	if !ok {
		return nil
	}
	if _, err := tx.ExecContext(ctx, q, requestID); err != nil {
		return fmt.Errorf("kosongkan tanda tangan: %w", err)
	}
	return nil
}

func (r *ActionRepository) Inbox(ctx context.Context, employeeID string, limit, offset int) ([]InboxItem, int, error) {
	if employeeID == "" {
		return nil, 0, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, sqlInbox, employeeID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("baca inbox: %w", err)
	}
	defer rows.Close()

	items := []InboxItem{}
	total := 0
	for rows.Next() {
		var it InboxItem
		var code string
		if err := rows.Scan(&it.RequestID, &code, &it.StepOrder, &it.Round, &it.SubmittedAt, &total); err != nil {
			return nil, 0, err
		}
		it.Code = StepCode(code)
		it.Label = it.Code.Label()
		items = append(items, it)
	}
	return items, total, rows.Err()
}

func (r *ActionRepository) PendingCount(ctx context.Context, employeeID string) (int, error) {
	if employeeID == "" {
		return 0, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, sqlPendingCount, employeeID).Scan(&n)
	return n, err
}
func (r *ActionRepository) Progress(ctx context.Context, requestID, viewerEmployeeID string) (Progress, error) {
	p := Progress{RequestID: requestID, Steps: []Step{}}

	var status string
	err := r.db.QueryRowContext(ctx, sqlProgressRequest, requestID).Scan(&status, &p.Round)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrRequestNotFound
	}
	if err != nil {
		return p, err
	}
	p.Status = RequestStatus(status)
	if p.Status == "" {
		return p, nil
	}

	rows, err := r.db.QueryContext(ctx, sqlProgressSteps, requestID, p.Round)
	if err != nil {
		return p, fmt.Errorf("baca langkah: %w", err)
	}
	defer rows.Close()

	index := map[string]int{}
	for rows.Next() {
		var s Step
		var code, st string
		var actedEmp, actedUser sql.NullString
		var actedAt sql.NullTime
		if err := rows.Scan(&s.ID, &code, &s.StepOrder, &st,
			&actedEmp, &actedUser, &s.ActedByNama, &s.ActedByJabatan, &actedAt, &s.Note, &s.CarriedOver, &s.CreatedAt); err != nil {
			return p, err
		}
		s.RequestID = requestID
		s.Round = p.Round
		s.Code = StepCode(code)
		s.Label = s.Code.Label()
		s.Status = StepStatus(st)
		s.Approvers = []StepApprover{}
		if actedEmp.Valid {
			v := actedEmp.String
			s.ActedByEmployeeID = &v
		}
		if actedUser.Valid {
			v := actedUser.String
			s.ActedByUserID = &v
		}
		if actedAt.Valid {
			t := actedAt.Time
			s.ActedAt = &t
		}
		index[s.ID] = len(p.Steps)
		p.Steps = append(p.Steps, s)
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	rows.Close()

	arows, err := r.db.QueryContext(ctx, sqlProgressApprovers, requestID, p.Round)
	if err != nil {
		return p, fmt.Errorf("baca pejabat: %w", err)
	}
	defer arows.Close()
	for arows.Next() {
		var stepID string
		var a StepApprover
		if err := arows.Scan(&stepID, &a.EmployeeID, &a.Nama, &a.Jabatan); err != nil {
			return p, err
		}
		if i, ok := index[stepID]; ok {
			p.Steps[i].Approvers = append(p.Steps[i].Approvers, a)
		}
	}
	if err := arows.Err(); err != nil {
		return p, err
	}

	if cur := p.Current(); cur != nil {
		p.CurrentStep = cur.StepOrder
		p.CanAct = cur.CanAct(viewerEmployeeID)
	}
	return p, nil
}
