package notify

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Mailer — pengirim email. Implementasi: GraphMailer (Microsoft 365, OAuth), SMTPMailer (relay / SMTP AUTH)
// dan LogMailer (dev).
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

func NewMailer(cfg Config) (Mailer, error) {
	switch cfg.Driver {
	case "graph":
		return NewGraphMailer(cfg)
	case "smtp":
		if cfg.SMTPHost == "" || cfg.From == "" {
			return nil, errors.New("MAIL_DRIVER=smtp butuh SMTP_HOST dan SMTP_FROM")
		}
		if NormalizeEmail(cfg.From) == "" {
			return nil, fmt.Errorf("SMTP_FROM tidak valid: %q", cfg.From)
		}
		return &SMTPMailer{cfg: cfg}, nil
	case "log", "off", "":
		return LogMailer{}, nil
	}
	return nil, fmt.Errorf("MAIL_DRIVER tidak dikenal: %q (pakai graph | smtp | log | off)", cfg.Driver)
}

// ---------- LogMailer ----------

type LogMailer struct{}

func (LogMailer) Send(_ context.Context, m Message) error {
	log.Printf("[mail:log] to=%s subject=%q (%d byte html)", m.To, m.Subject, len(m.HTML))
	return nil
}

// ---------- SMTPMailer ----------

type SMTPMailer struct {
	cfg Config
}

const smtpTimeout = 30 * time.Second

func (s *SMTPMailer) Send(ctx context.Context, m Message) error {
	to := NormalizeEmail(m.To)
	if to == "" {
		return fmt.Errorf("alamat penerima tidak valid: %q", m.To)
	}
	host := s.cfg.SMTPHost
	addr := net.JoinHostPort(host, s.cfg.SMTPPort)

	deadline := time.Now().Add(smtpTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := &net.Dialer{Deadline: deadline}
	tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	var err error
	if s.cfg.TLSMode == "tls" { // implicit TLS, biasanya port 465
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("koneksi SMTP %s: %w", addr, err)
	}
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("salam SMTP: %w", err)
	}
	defer c.Close()

	if s.cfg.TLSMode != "tls" && s.cfg.TLSMode != "none" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("STARTTLS: %w", err)
			}
		} else if s.cfg.TLSMode == "starttls" {
			return errors.New("server SMTP tidak menawarkan STARTTLS")
		}
	}

	if s.cfg.SMTPUser != "" {
		ok, mechs := c.Extension("AUTH")
		if !ok {
			return errors.New("server SMTP tidak menerima AUTH; kosongkan SMTP_USERNAME untuk mode relay")
		}
		var auth smtp.Auth
		if strings.Contains(strings.ToUpper(mechs), "LOGIN") {
			auth = loginAuth{user: s.cfg.SMTPUser, pass: s.cfg.SMTPPass}
		} else {
			auth = smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, host)
		}
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("login SMTP: %w", err)
		}
	}

	from := NormalizeEmail(s.cfg.From)
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO %s: %w", to, err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(buildMIME(from, s.cfg.FromName, to, m.ToName, m.Subject, m.HTML)); err != nil {
		w.Close()
		return fmt.Errorf("tulis isi email: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("kirim isi email: %w", err)
	}
	return c.Quit()
}

func stripCRLF(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

func buildMIME(from, fromName, to, toName, subject, html string) []byte {
	var b strings.Builder
	fromAddr := mail.Address{Name: stripCRLF(fromName), Address: from}
	toAddr := mail.Address{Name: stripCRLF(toName), Address: to}

	domain := "localhost"
	if i := strings.LastIndex(from, "@"); i >= 0 {
		domain = from[i+1:]
	}
	rnd := make([]byte, 12)
	_, _ = rand.Read(rnd)

	b.WriteString("From: " + fromAddr.String() + "\r\n")
	b.WriteString("To: " + toAddr.String() + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", stripCRLF(subject)) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + hex.EncodeToString(rnd) + "@" + domain + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")

	enc := base64.StdEncoding.EncodeToString([]byte(html))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return []byte(b.String())
}

// loginAuth — AUTH LOGIN (dipakai smtp.office365.com). Hanya di atas TLS.
type loginAuth struct{ user, pass string }

func (a loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, errors.New("AUTH LOGIN ditolak: koneksi belum terenkripsi")
	}
	return "LOGIN", nil, nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.user), nil
	case "password:":
		return []byte(a.pass), nil
	}
	return nil, fmt.Errorf("tantangan AUTH LOGIN tidak dikenal: %q", fromServer)
}
