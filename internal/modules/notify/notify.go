package notify

import (
	"errors"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

type Event string

const (
	EventStepActive Event = "step_active"
	EventRejected   Event = "rejected"
	EventApproved   Event = "approved"
	EventTest       Event = "test"
)

type OutboxStatus string

const (
	StatusPending OutboxStatus = "pending"
	StatusSending OutboxStatus = "sending"
	StatusSent    OutboxStatus = "sent"
	StatusFailed  OutboxStatus = "failed"
)

var ErrOutboxNotFound = errors.New("notifikasi tidak ditemukan atau tidak bisa dikirim ulang")

type Message struct {
	To      string
	ToName  string
	Subject string
	HTML    string
}

type Outbox struct {
	ID             int64        `json:"id"`
	RequestID      string       `json:"requestId"`
	Event          Event        `json:"event"`
	RecipientEmail string       `json:"recipientEmail"`
	RecipientName  string       `json:"recipientName"`
	Subject        string       `json:"subject"`
	Status         OutboxStatus `json:"status"`
	Attempts       int          `json:"attempts"`
	LastError      string       `json:"lastError,omitempty"`
	NextAttemptAt  time.Time    `json:"nextAttemptAt"`
	SentAt         *time.Time   `json:"sentAt,omitempty"`
	CreatedAt      time.Time    `json:"createdAt"`
}

type TestInput struct {
	To string `json:"to"`
}

type Config struct {
	Driver     string
	SMTPHost   string
	SMTPPort   string
	SMTPUser   string
	SMTPPass   string
	TLSMode    string
	From       string
	FromName   string
	BaseURL    string
	RedirectTo string

	GraphTenantID     string
	GraphClientID     string
	GraphClientSecret string

	ApprovedCC  []string
	Poll        time.Duration
	MaxAttempts int
	BatchSize   int
}

func ConfigFromEnv() Config {
	c := Config{
		Driver:      strings.ToLower(env("MAIL_DRIVER", "log")),
		SMTPHost:    env("SMTP_HOST", ""),
		SMTPPort:    env("SMTP_PORT", "587"),
		SMTPUser:    env("SMTP_USERNAME", ""),
		SMTPPass:    os.Getenv("SMTP_PASSWORD"),
		TLSMode:     strings.ToLower(env("SMTP_TLS", "auto")),
		From:        env("SMTP_FROM", ""),
		FromName:    env("SMTP_FROM_NAME", "ITR Platform"),
		BaseURL:     strings.TrimRight(env("APP_BASE_URL", "http://localhost:3030"), "/"),
		RedirectTo:  env("MAIL_REDIRECT_TO", ""),
		Poll:        time.Duration(envInt("NOTIFY_POLL_SECONDS", 15)) * time.Second,
		MaxAttempts: envInt("NOTIFY_MAX_ATTEMPTS", 6),
		BatchSize:   envInt("NOTIFY_BATCH", 20),

		GraphTenantID:     env("GRAPH_TENANT_ID", ""),
		GraphClientID:     env("GRAPH_CLIENT_ID", ""),
		GraphClientSecret: os.Getenv("GRAPH_CLIENT_SECRET"),
	}
	for _, e := range strings.Split(os.Getenv("NOTIFY_APPROVED_CC"), ",") {
		if e = NormalizeEmail(e); e != "" {
			c.ApprovedCC = append(c.ApprovedCC, e)
		}
	}
	if c.Poll < 5*time.Second {
		c.Poll = 5 * time.Second
	}
	if c.MaxAttempts < 1 {
		c.MaxAttempts = 1
	}
	if c.BatchSize < 1 || c.BatchSize > 100 {
		c.BatchSize = 20
	}
	return c
}

// Enabled — false bila MAIL_DRIVER=off: tidak ada email yang diantrikan.
func (c Config) Enabled() bool { return c.Driver != "off" }

func (c Config) RequestURL(requestID string) string {
	return c.BaseURL + "/dashboard/requests/" + requestID
}

// NormalizeEmail mengembalikan "" bila alamat tidak valid.
func NormalizeEmail(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || strings.ContainsAny(s, "\r\n") {
		return ""
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return ""
	}
	return s
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(k))); err == nil {
		return v
	}
	return def
}
