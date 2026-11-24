package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type GraphMailer struct {
	cfg      Config
	http     *http.Client
	tokenURL string
	apiBase  string

	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewGraphMailer(cfg Config) (*GraphMailer, error) {
	var missing []string
	if cfg.GraphTenantID == "" {
		missing = append(missing, "GRAPH_TENANT_ID")
	}
	if cfg.GraphClientID == "" {
		missing = append(missing, "GRAPH_CLIENT_ID")
	}
	if cfg.GraphClientSecret == "" {
		missing = append(missing, "GRAPH_CLIENT_SECRET")
	}
	if NormalizeEmail(cfg.From) == "" {
		missing = append(missing, "SMTP_FROM (mailbox pengirim)")
	}
	if len(missing) > 0 {
		return nil, errors.New("MAIL_DRIVER=graph butuh: " + strings.Join(missing, ", "))
	}
	return &GraphMailer{
		cfg:      cfg,
		http:     &http.Client{Timeout: 30 * time.Second}, // ikut HTTPS_PROXY bila ada
		tokenURL: "https://login.microsoftonline.com/" + url.PathEscape(cfg.GraphTenantID) + "/oauth2/v2.0/token",
		apiBase:  "https://graph.microsoft.com/v1.0",
	}, nil
}

func (g *GraphMailer) accessToken(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.token != "" && time.Now().Before(g.expires) {
		return g.token, nil
	}

	form := url.Values{
		"client_id":     {g.cfg.GraphClientID},
		"client_secret": {g.cfg.GraphClientSecret},
		"scope":         {"https://graph.microsoft.com/.default"},
		"grant_type":    {"client_credentials"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("minta token Microsoft: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &tr)
	if resp.StatusCode != http.StatusOK || tr.AccessToken == "" {
		msg := tr.Description
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		if i := strings.Index(msg, "\r\n"); i > 0 { // buang trace id panjang
			msg = msg[:i]
		}
		return "", fmt.Errorf("token Microsoft ditolak (%d %s): %s", resp.StatusCode, tr.Error, msg)
	}
	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	g.token = tr.AccessToken
	g.expires = time.Now().Add(ttl - 5*time.Minute)
	return g.token, nil
}

type graphAddr struct {
	EmailAddress struct {
		Address string `json:"address"`
		Name    string `json:"name,omitempty"`
	} `json:"emailAddress"`
}

func (g *GraphMailer) Send(ctx context.Context, m Message) error {
	to := NormalizeEmail(m.To)
	if to == "" {
		return fmt.Errorf("alamat penerima tidak valid: %q", m.To)
	}
	var rcpt graphAddr
	rcpt.EmailAddress.Address = to
	rcpt.EmailAddress.Name = stripCRLF(m.ToName)

	payload := map[string]any{
		"message": map[string]any{
			"subject":      stripCRLF(m.Subject),
			"body":         map[string]string{"contentType": "HTML", "content": m.HTML},
			"toRecipients": []graphAddr{rcpt},
		},
		"saveToSentItems": false,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	sender := NormalizeEmail(g.cfg.From)
	endpoint := g.apiBase + "/users/" + url.PathEscape(sender) + "/sendMail"

	for attempt := 0; attempt < 2; attempt++ {
		tok, err := g.accessToken(ctx)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")

		resp, err := g.http.Do(req)
		if err != nil {
			return fmt.Errorf("kirim lewat Graph: %w", err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
			return nil
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			g.mu.Lock()
			g.token = ""
			g.mu.Unlock()
			continue
		}

		var ge struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &ge)
		msg := ge.Error.Message
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		hint := ""
		switch resp.StatusCode {
		case http.StatusForbidden:
			hint = " — cek izin Application Mail.Send sudah di-grant admin consent dan mailbox pengirim termasuk yang diizinkan"
		case http.StatusNotFound:
			hint = " — mailbox pengirim (SMTP_FROM) tidak ditemukan di tenant"
		}
		return fmt.Errorf("Graph sendMail %d %s: %s%s", resp.StatusCode, ge.Error.Code, msg, hint)
	}
	return errors.New("Graph sendMail: token ditolak berulang")
}
