package onedbsync

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	ErrSyncRunning       = errors.New("sinkronisasi karyawan sedang berjalan")
	ErrSyncNotConfigured = errors.New("EMPLOYEE_API_URL belum diatur")
)

type EmployeeSyncConfig struct {
	URL         string
	SC          string
	Token       string
	Company     string
	InsecureTLS bool
	Interval    time.Duration
}

func EmployeeSyncConfigFromEnv() EmployeeSyncConfig {
	company := strings.TrimSpace(os.Getenv("EMPLOYEE_API_COMPANY"))
	if company == "" {
		company = "PT UNGGUL DINAMIKA UTAMA"
	}
	interval, err := time.ParseDuration(os.Getenv("EMPLOYEE_SYNC_INTERVAL"))
	if err != nil {
		interval = 0
	}
	return EmployeeSyncConfig{
		URL:         strings.TrimSpace(os.Getenv("EMPLOYEE_API_URL")),
		SC:          strings.TrimSpace(os.Getenv("EMPLOYEE_API_SC")),
		Token:       strings.TrimSpace(os.Getenv("EMPLOYEE_API_TOKEN")),
		Company:     company,
		InsecureTLS: os.Getenv("EMPLOYEE_API_INSECURE_TLS") == "true",
		Interval:    interval,
	}
}

type EmployeeSyncHandler struct {
	repo   *EmployeeSyncRepository
	cfg    EmployeeSyncConfig
	client *http.Client
	mu     sync.Mutex
}

func NewEmployeeSyncHandler(repo *EmployeeSyncRepository, cfg EmployeeSyncConfig) *EmployeeSyncHandler {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureTLS {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &EmployeeSyncHandler{repo: repo, cfg: cfg, client: &http.Client{Timeout: 2 * time.Minute, Transport: tr}}
}

func (h *EmployeeSyncHandler) Start(ctx context.Context) {
	if h.cfg.URL == "" {
		log.Println("[employee-sync] nonaktif: EMPLOYEE_API_URL kosong")
		return
	}
	go func() {
		h.runAndLog()
		if h.cfg.Interval <= 0 {
			return
		}
		t := time.NewTicker(h.cfg.Interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				h.runAndLog()
			}
		}
	}()
}

func (h *EmployeeSyncHandler) runAndLog() {
	ins, upd, err := h.sync()
	if err != nil {
		log.Printf("[employee-sync] gagal: %v", err)
		return
	}
	log.Printf("[employee-sync] selesai: %d baru, %d diperbarui", ins, upd)
}

func (h *EmployeeSyncHandler) Sync(c *gin.Context) {
	ins, upd, err := h.sync()
	if err != nil {
		code := http.StatusBadGateway
		if errors.Is(err, ErrSyncRunning) {
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"inserted": ins, "updated": upd})
}

var errSyncRunning = errors.New("Sinkronisasi sedang berjalan")

func (h *EmployeeSyncHandler) sync() (inserted, updated int, err error) {
	if h.cfg.URL == "" {
		return 0, 0, errors.New("EMPLOYEE_API_URL belum diatur")
	}
	if !h.mu.TryLock() {
		return 0, 0, errSyncRunning
	}
	defer h.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	list, err := h.fetch(ctx)
	if err != nil {
		return 0, 0, err
	}
	rows := mapEmployees(list, h.cfg.Company, time.Now())
	if len(rows) == 0 {
		return 0, 0, fmt.Errorf("Tidak ada data karyawan %q di data API (%d baris diterima)", h.cfg.Company, len(list))
	}
	return h.repo.SyncUpsert(ctx, rows)
}

func (h *EmployeeSyncHandler) fetch(ctx context.Context) ([]EmployeeAPI, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.cfg.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("sc", h.cfg.SC)
	req.Header.Set("Authorization", h.cfg.Token)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API membalas HTTP %d", resp.StatusCode)
	}
	return parseEmployees(body)
}

func parseEmployees(body []byte) ([]EmployeeAPI, error) {
	var wrapped struct {
		Data []EmployeeAPI `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Data != nil {
		return wrapped.Data, nil
	}
	var list []EmployeeAPI
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, errors.New("format respons API tidak dikenali")
	}
	return list, nil
}

func mapEmployees(list []EmployeeAPI, company string, now time.Time) []EmployeeSync {
	want := normalize(company)
	seen := map[string]int{}
	var out []EmployeeSync

	for _, e := range list {
		if normalize(string(e.NamaPerusahaan)) != want {
			continue
		}
		nik, nama := clean(e.NoNIK), clean(e.NamaLengkap)
		if nik == "" || nama == "" {
			continue
		}
		row := EmployeeSync{
			NIK: nik, Nama: nama,
			Jabatan: clean(e.Posisi), Departemen: clean(e.Depart), Level: clean(e.Level),
			StaffType: staffType(string(e.Tipe)),
			IsActive:  isActive(e.TglNonaktif, now),
		}
		if i, ok := seen[nik]; ok {
			out[i] = row
			continue
		}
		seen[nik] = len(out)
		out = append(out, row)
	}
	return out
}

func normalize(s string) string {
	s = strings.ToUpper(strings.ReplaceAll(s, ".", " "))
	return strings.Join(strings.Fields(s), " ")
}

func clean(s Flexstring) string {
	v := strings.TrimSpace(html.UnescapeString(string(s)))
	if v == "-" {
		return ""
	}
	return v
}

func staffType(tipe string) string {
	if normalize(tipe) == "STAFF" {
		return "staff"
	}
	return "non_staff"
}

func isActive(tglNonaktif Flexstring, now time.Time) bool {
	v := clean(tglNonaktif)
	if len(v) < 10 {
		return true
	}
	t, err := time.Parse("2006-01-02", v[:10])
	if err != nil || t.Year() <= 1970 {
		return true
	}
	return t.After(now)
}
