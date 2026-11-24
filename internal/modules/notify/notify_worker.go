package notify

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

type Worker struct {
	repo   *Repository
	mailer Mailer
	cfg    Config
	mu     sync.Mutex
}

func NewWorker(repo *Repository, mailer Mailer, cfg Config) *Worker {
	return &Worker{repo: repo, mailer: mailer, cfg: cfg}
}

func (w *Worker) Start(ctx context.Context) {
	if !w.cfg.Enabled() {
		log.Println("[notify] MAIL_DRIVER=off — pengiriman email nonaktif")
		return
	}
	log.Printf("[notify] worker aktif: driver=%s, tiap %s", w.cfg.Driver, w.cfg.Poll)
	go func() {
		t := time.NewTicker(w.cfg.Poll)
		defer t.Stop()
		for {
			w.RunOnce(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func (w *Worker) RunOnce(ctx context.Context) (sent, failed int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for {
		jobs, err := w.repo.claim(ctx, w.cfg.BatchSize)
		if err != nil {
			log.Printf("[notify] ambil antrian: %v", err)
			return
		}
		for _, j := range jobs {
			msg := Message{To: j.Email, ToName: j.Name, Subject: j.Subject, HTML: j.HTML}
			targets := []string{j.Email}
			if w.cfg.RedirectTo != "" { // mode uji: semua email dialihkan ke alamat di MAIL_REDIRECT_TO
				msg.Subject = "[UJI → " + j.Email + "] " + j.Subject
				targets = targets[:0]
				for _, a := range strings.Split(w.cfg.RedirectTo, ",") {
					if a = NormalizeEmail(a); a != "" {
						targets = append(targets, a)
					}
				}
			}
			var err error
			for _, to := range targets {
				msg.To = to
				sendCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				err = w.mailer.Send(sendCtx, msg)
				cancel()
				if err != nil {
					break
				}
			}

			if err != nil {
				failed++
				log.Printf("[notify] gagal kirim #%d ke %s (percobaan %d): %v", j.ID, msg.To, j.Attempts, err)
				if mErr := w.repo.markFailed(ctx, j, err); mErr != nil {
					log.Printf("[notify] tandai gagal #%d: %v", j.ID, mErr)
				}
				continue
			}
			sent++
			if mErr := w.repo.markSent(ctx, j.ID); mErr != nil {
				log.Printf("[notify] tandai terkirim #%d: %v", j.ID, mErr)
			}
		}
		if len(jobs) < w.cfg.BatchSize || ctx.Err() != nil {
			return
		}
	}
}
