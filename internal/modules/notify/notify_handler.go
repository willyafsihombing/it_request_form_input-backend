package notify

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"it-request-backend/internal/shared/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Repo   *Repository
	Mailer Mailer
	Worker *Worker
	Cfg    Config
}

func NewHandler(repo *Repository, mailer Mailer, worker *Worker, cfg Config) *Handler {
	return &Handler{Repo: repo, Mailer: mailer, Worker: worker, Cfg: cfg}
}

func (h *Handler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, total, err := h.Repo.List(c.Request.Context(), c.Query("status"), c.Query("requestId"), limit, offset)
	if err != nil {
		log.Printf("[notify] list: %v", err)
		utils.InternalError(c, "Gagal membaca antrian email")
		return
	}
	utils.OK(c, gin.H{"items": items, "total": total, "driver": h.Cfg.Driver, "redirectTo": h.Cfg.RedirectTo})
}

func (h *Handler) Retry(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "ID tidak valid")
		return
	}
	if err := h.Repo.Retry(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrOutboxNotFound) {
			utils.NotFound(c, err.Error())
			return
		}
		log.Printf("[notify] retry #%d: %v", id, err)
		utils.InternalError(c, "Gagal mengantrikan ulang")
		return
	}
	utils.OK(c, gin.H{"queued": true})
}

func (h *Handler) RunNow(c *gin.Context) {
	if !h.Cfg.Enabled() {
		c.JSON(http.StatusConflict, gin.H{"error": "Pengiriman email nonaktif (MAIL_DRIVER=off)"})
		return
	}
	sent, failed := h.Worker.RunOnce(c.Request.Context())
	utils.OK(c, gin.H{"sent": sent, "failed": failed})
}

func (h *Handler) Test(c *gin.Context) {
	var in TestInput
	_ = c.ShouldBindJSON(&in)

	to, name := NormalizeEmail(in.To), ""
	if to == "" {
		cl := utils.CurrentClaims(c)
		if cl == nil || cl.ID == "" {
			utils.Forbidden(c, "Akun tidak valid")
			return
		}
		email, nm, err := h.Repo.UserContact(c.Request.Context(), cl.ID)
		if err != nil {
			utils.InternalError(c, "Gagal membaca email akun")
			return
		}
		to, name = NormalizeEmail(email), nm
	}
	if to == "" {
		utils.BadRequest(c, "Alamat email tujuan kosong atau tidak valid")
		return
	}

	html, err := render(mailData{
		Heading:    "Uji Coba Email",
		Greeting:   "Halo,",
		Intro:      "Jika Anda menerima email ini, konfigurasi pengiriman email ITR Platform sudah berjalan.",
		FormNumber: "-", Title: "Email uji coba", Requester: "-",
		URL:        h.Cfg.BaseURL + "/dashboard",
		ButtonText: "Buka ITR Platform",
		Accent:     "#0284c7",
	})
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}
	if err := h.Mailer.Send(c.Request.Context(), Message{To: to, ToName: name, Subject: "[ITR] Uji coba email", HTML: html}); err != nil {
		log.Printf("[notify] test ke %s: %v", to, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Gagal kirim: " + err.Error()})
		return
	}
	utils.OK(c, gin.H{"sent": true, "to": to, "driver": h.Cfg.Driver})
}
