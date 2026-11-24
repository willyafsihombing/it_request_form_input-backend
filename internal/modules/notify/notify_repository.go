package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type Repository struct {
	db    *sql.DB
	cfg   Config
	label func(code string) string
}

func NewRepository(db *sql.DB, cfg Config, stepLabel func(code string) string) *Repository {
	if stepLabel == nil {
		stepLabel = func(c string) string { return c }
	}
	return &Repository{db: db, cfg: cfg, label: stepLabel}
}

type reqInfo struct {
	ID, FormNumber, Title, Requester, Department string
	RequesterEmail                               string
	Round                                        int
}

const sqlReqInfo = `
SELECT q.id::text,
       COALESCE(q.form_number, ''),
       COALESCE(q.request_title, ''),
       COALESCE(NULLIF(q.req_name, ''), u.full_name, ''),
       COALESCE(e.departemen, ''),
       COALESCE(u.email_addrs, ''),
       COALESCE(q.approval_round, 0)
FROM requests q
LEFT JOIN users u ON u.id::text = q.created_by_user_id::text AND u.deleted_at IS NULL
LEFT JOIN employees e ON e.id = u.employee_id
WHERE q.id::text = $1`

const sqlActiveStepRecipients = `
SELECT s.code, s.step_order, COALESCE(u.email_addrs, ''), COALESCE(a.nama, '')
FROM approval_steps s
JOIN approval_step_approvers a ON a.step_id = s.id
JOIN users u ON u.employee_id = a.employee_id AND u.deleted_at IS NULL
WHERE s.request_id::text = $1 AND s.round = $2 AND s.status = 'active'`

const sqlRejectedStep = `
SELECT code, step_order, COALESCE(acted_by_nama, ''), COALESCE(note, '')
FROM approval_steps
WHERE request_id::text = $1 AND round = $2 AND status = 'rejected'
ORDER BY step_order DESC LIMIT 1`

const sqlInsertOutbox = `
INSERT INTO notification_outbox
    (request_id, event, dedupe_key, recipient_email, recipient_name, subject, body_html)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (dedupe_key) DO NOTHING`

func loadReqInfo(ctx context.Context, tx *sql.Tx, requestID string) (reqInfo, error) {
	var ri reqInfo
	err := tx.QueryRowContext(ctx, sqlReqInfo, requestID).Scan(
		&ri.ID, &ri.FormNumber, &ri.Title, &ri.Requester, &ri.Department, &ri.RequesterEmail, &ri.Round)
	return ri, err
}

func (r *Repository) safely(ctx context.Context, tx *sql.Tx, what, requestID string, fn func() error) error {
	if !r.cfg.Enabled() || tx == nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT notify_enqueue"); err != nil {
		log.Printf("[notify] %s %s: savepoint: %v", what, requestID, err)
		return nil
	}
	if err := fn(); err != nil {
		log.Printf("[notify] %s %s gagal diantrikan: %v", what, requestID, err)
		if _, rbErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT notify_enqueue"); rbErr != nil {
			return fmt.Errorf("rollback savepoint notifikasi: %w", rbErr) // tx rusak, biarkan gagal
		}
		return nil
	}
	_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT notify_enqueue")
	return nil
}

func (r *Repository) insert(ctx context.Context, tx *sql.Tx, requestID string, ev Event, key, email, name, subject string, d mailData) error {
	html, err := render(d)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, sqlInsertOutbox, requestID, string(ev), key, email, name, subject, html)
	return err
}

func (r *Repository) StepActivated(ctx context.Context, tx *sql.Tx, requestID string) error {
	return r.safely(ctx, tx, "step_active", requestID, func() error {
		ri, err := loadReqInfo(ctx, tx, requestID)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, sqlActiveStepRecipients, requestID, ri.Round)
		if err != nil {
			return err
		}
		type rcpt struct {
			code      string
			order     int
			email, nm string
		}
		var list []rcpt
		for rows.Next() {
			var x rcpt
			if err := rows.Scan(&x.code, &x.order, &x.email, &x.nm); err != nil {
				rows.Close()
				return err
			}
			list = append(list, x)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		sent := 0
		for _, x := range list {
			email := NormalizeEmail(x.email)
			if email == "" {
				log.Printf("[notify] %s: approver %q tidak punya email valid", ri.FormNumber, x.nm)
				continue
			}
			stepLabel := r.label(x.code)
			key := fmt.Sprintf("step_active:%s:r%d:s%d:%s", ri.ID, ri.Round, x.order, email)
			subject := subjectFor(EventStepActive, ri.FormNumber, ri.Title)
			d := mailData{
				Heading:    "Menunggu Persetujuan Anda",
				Greeting:   "Yth. " + x.nm + ",",
				Intro:      "Ada IT Request yang menunggu persetujuan Anda sebagai " + stepLabel + ". Silakan buka aplikasi untuk meninjau lalu Setujui atau Tolak.",
				FormNumber: ri.FormNumber, Title: ri.Title, Requester: ri.Requester, Department: ri.Department,
				StepLabel:  stepLabel,
				URL:        r.cfg.RequestURL(ri.ID),
				ButtonText: "Buka Request",
				Accent:     "#0284c7",
			}
			if err := r.insert(ctx, tx, ri.ID, EventStepActive, key, email, x.nm, subject, d); err != nil {
				return err
			}
			sent++
		}
		if sent == 0 && len(list) > 0 {
			log.Printf("[notify] %s: tidak ada approver langkah aktif yang punya email", ri.FormNumber)
		}
		return nil
	})
}

func (r *Repository) RequestRejected(ctx context.Context, tx *sql.Tx, requestID string) error {
	return r.safely(ctx, tx, "rejected", requestID, func() error {
		ri, err := loadReqInfo(ctx, tx, requestID)
		if err != nil {
			return err
		}
		email := NormalizeEmail(ri.RequesterEmail)
		if email == "" {
			log.Printf("[notify] %s: requester tidak punya email valid", ri.FormNumber)
			return nil
		}
		var code, by, note string
		var order int
		if err := tx.QueryRowContext(ctx, sqlRejectedStep, requestID, ri.Round).Scan(&code, &order, &by, &note); err != nil {
			return err
		}
		key := fmt.Sprintf("rejected:%s:r%d:s%d:%s", ri.ID, ri.Round, order, email)
		d := mailData{
			Heading:    "Request Ditolak",
			Greeting:   "Yth. " + ri.Requester + ",",
			Intro:      "IT Request Anda ditolak. Perbaiki sesuai catatan di bawah lalu ajukan ulang melalui aplikasi.",
			FormNumber: ri.FormNumber, Title: ri.Title, Requester: ri.Requester, Department: ri.Department,
			StepLabel:  r.label(code),
			ActedBy:    by,
			NoteLabel:  "Alasan penolakan",
			Note:       note,
			URL:        r.cfg.RequestURL(ri.ID),
			ButtonText: "Lihat & Ajukan Ulang",
			Accent:     "#dc2626",
		}
		return r.insert(ctx, tx, ri.ID, EventRejected, key, email, ri.Requester,
			subjectFor(EventRejected, ri.FormNumber, ri.Title), d)
	})
}

func (r *Repository) RequestApproved(ctx context.Context, tx *sql.Tx, requestID string) error {
	return r.safely(ctx, tx, "approved", requestID, func() error {
		ri, err := loadReqInfo(ctx, tx, requestID)
		if err != nil {
			return err
		}
		type rcpt struct{ email, name string }
		var list []rcpt
		if e := NormalizeEmail(ri.RequesterEmail); e != "" {
			list = append(list, rcpt{e, ri.Requester})
		} else {
			log.Printf("[notify] %s: requester tidak punya email valid", ri.FormNumber)
		}
		for _, cc := range r.cfg.ApprovedCC {
			list = append(list, rcpt{cc, "Tim IT"})
		}
		for _, x := range list {
			greet := "Yth. " + x.name + ","
			key := fmt.Sprintf("approved:%s:r%d:%s", ri.ID, ri.Round, x.email)
			d := mailData{
				Heading:    "Request Disetujui",
				Greeting:   greet,
				Intro:      "Seluruh tahap persetujuan IT Request ini telah selesai. Request akan ditindaklanjuti oleh tim IT.",
				FormNumber: ri.FormNumber, Title: ri.Title, Requester: ri.Requester, Department: ri.Department,
				URL:        r.cfg.RequestURL(ri.ID),
				ButtonText: "Lihat Request",
				Accent:     "#16a34a",
			}
			if err := r.insert(ctx, tx, ri.ID, EventApproved, key, x.email, x.name,
				subjectFor(EventApproved, ri.FormNumber, ri.Title), d); err != nil {
				return err
			}
		}
		return nil
	})
}

type job struct {
	ID       int64
	Email    string
	Name     string
	Subject  string
	HTML     string
	Attempts int
}

const sqlClaim = `
UPDATE notification_outbox o
SET status = 'sending', locked_at = NOW(), attempts = o.attempts + 1
WHERE o.id IN (
    SELECT id FROM notification_outbox
    WHERE (status = 'pending' AND next_attempt_at <= NOW())
       OR (status = 'sending' AND locked_at < NOW() - INTERVAL '10 minutes')
    ORDER BY id
    LIMIT $1
    FOR UPDATE SKIP LOCKED)
RETURNING o.id, o.recipient_email, o.recipient_name, o.subject, o.body_html, o.attempts`

func (r *Repository) claim(ctx context.Context, n int) ([]job, error) {
	rows, err := r.db.QueryContext(ctx, sqlClaim, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.ID, &j.Email, &j.Name, &j.Subject, &j.HTML, &j.Attempts); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *Repository) markSent(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE notification_outbox
		SET status = 'sent', sent_at = NOW(), locked_at = NULL, last_error = NULL
		WHERE id = $1`, id)
	return err
}

func (r *Repository) markFailed(ctx context.Context, j job, sendErr error) error {
	msg := sendErr.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	status := StatusPending
	if j.Attempts >= r.cfg.MaxAttempts {
		status = StatusFailed
	}
	delay := time.Minute << min(j.Attempts-1, 6)
	if delay > time.Hour {
		delay = time.Hour
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE notification_outbox
		SET status = $2, last_error = $3, locked_at = NULL,
		    next_attempt_at = NOW() + make_interval(secs => $4)
		WHERE id = $1`, j.ID, string(status), msg, delay.Seconds())
	return err
}

func (r *Repository) List(ctx context.Context, status, requestID string, limit, offset int) ([]Outbox, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var where []string
	var args []any
	if status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if requestID != "" {
		args = append(args, requestID)
		where = append(where, fmt.Sprintf("request_id = $%d", len(args)))
	}
	q := `SELECT id, request_id, event, recipient_email, recipient_name, subject, status,
	             attempts, COALESCE(last_error, ''), next_attempt_at, sent_at, created_at,
	             COUNT(*) OVER()
	      FROM notification_outbox`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Outbox{}
	total := 0
	for rows.Next() {
		var o Outbox
		var ev, st string
		var sentAt sql.NullTime
		if err := rows.Scan(&o.ID, &o.RequestID, &ev, &o.RecipientEmail, &o.RecipientName, &o.Subject,
			&st, &o.Attempts, &o.LastError, &o.NextAttemptAt, &sentAt, &o.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		o.Event, o.Status = Event(ev), OutboxStatus(st)
		if sentAt.Valid {
			t := sentAt.Time
			o.SentAt = &t
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
}

func (r *Repository) Retry(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notification_outbox
		SET status = 'pending', attempts = 0, next_attempt_at = NOW(), locked_at = NULL, last_error = NULL
		WHERE id = $1 AND status IN ('failed', 'sent')`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrOutboxNotFound
	}
	return nil
}

func (r *Repository) UserContact(ctx context.Context, userID string) (email, name string, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(email_addrs, ''), COALESCE(full_name, '')
		FROM users WHERE id::text = $1 AND deleted_at IS NULL`, userID).Scan(&email, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return email, name, err
}
