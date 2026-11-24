package approve

import (
	"context"
	"database/sql"
)

// Notifier dipanggil DI DALAM transaksi approval untuk mengantrikan email.
// Implementasi: notify.Repository (paket notify tidak meng-import approve, jadi tidak ada siklus import).
type Notifier interface {
	StepActivated(ctx context.Context, tx *sql.Tx, requestID string) error
	RequestRejected(ctx context.Context, tx *sql.Tx, requestID string) error
	RequestApproved(ctx context.Context, tx *sql.Tx, requestID string) error
}

// UseNotifier memasang notifikasi email. nil = tanpa email.
func (r *ActionRepository) UseNotifier(n Notifier) *ActionRepository {
	r.notify = n
	return r
}

// NotifyStepActivated dipakai modul itrequest setelah rantai dibuat / diajukan ulang.
func (r *ActionRepository) NotifyStepActivated(ctx context.Context, tx *sql.Tx, requestID string) error {
	if r == nil || r.notify == nil {
		return nil
	}
	return r.notify.StepActivated(ctx, tx, requestID)
}
