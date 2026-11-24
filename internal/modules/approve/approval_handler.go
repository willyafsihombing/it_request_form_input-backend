package approve

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const maxNoteRunes = 1000

type Handler struct {
	Actions *ActionRepository
	Routes  *RouteRepository
	Chains  *ChainRepository

	UserID func(c *gin.Context) string
}

func NewHandler(a *ActionRepository, r *RouteRepository, ch *ChainRepository, userID func(c *gin.Context) string) *Handler {
	return &Handler{Actions: a, Routes: r, Chains: ch, UserID: userID}
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func statusFor(err error) (int, string) {
	switch {
	case errors.Is(err, ErrRequestNotFound), errors.Is(err, ErrRouteNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, ErrEmployeeNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, ErrNotApprover):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, ErrNotInReview), errors.Is(err, ErrNoActiveStep), errors.Is(err, ErrRouteExists):
		return http.StatusConflict, err.Error()
	case errors.Is(err, ErrNoteRequired):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, ErrNoSignature):
		return http.StatusUnprocessableEntity, "Unggah tanda tangan dulu di menu Tanda Tangan Saya"
	}
	log.Printf("approval: %v", err)
	return http.StatusInternalServerError, "Terjadi kesalahan pada server"
}

func failErr(c *gin.Context, err error) {
	status, msg := statusFor(err)
	fail(c, status, msg)
}

func (h *Handler) actor(c *gin.Context) (Actor, bool) {
	uid := h.UserID(c)
	if uid == "" {
		fail(c, http.StatusUnauthorized, "Sesi tidak valid, silakan login ulang")
		return Actor{}, false
	}
	emp, err := h.Actions.EmployeeIDByUser(c.Request.Context(), uid)
	if err != nil {
		failErr(c, err)
		return Actor{}, false
	}
	return Actor{UserID: uid, EmployeeID: emp}, true
}

func checkNote(c *gin.Context, note string) (string, bool) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNoteRunes {
		fail(c, http.StatusBadRequest, "Catatan terlalu panjang (maksimal "+strconv.Itoa(maxNoteRunes)+" karakter)")
		return "", false
	}
	return note, true
}

func intQuery(c *gin.Context, key string, def int) int {
	if v, err := strconv.Atoi(c.Query(key)); err == nil {
		return v
	}
	return def
}

func (h *Handler) Approve(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var in ApproveInput
	if err := c.ShouldBindJSON(&in); err != nil && !errors.Is(err, io.EOF) {
		fail(c, http.StatusBadRequest, "Format permintaan tidak valid")
		return
	}
	note, ok := checkNote(c, in.Note)
	if !ok {
		return
	}
	res, err := h.Actions.Approve(c.Request.Context(), c.Param("id"), actor, note)
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) Reject(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var in RejectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, ErrNoteRequired.Error())
		return
	}
	note, ok := checkNote(c, in.Note)
	if !ok {
		return
	}
	res, err := h.Actions.Reject(c.Request.Context(), c.Param("id"), actor, note)
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) Progress(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	p, err := h.Actions.Progress(c.Request.Context(), c.Param("id"), actor.EmployeeID)
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) Inbox(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	items, total, err := h.Actions.Inbox(c.Request.Context(), actor.EmployeeID,
		intQuery(c, "limit", 20), intQuery(c, "offset", 0))
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handler) InboxCount(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	n, err := h.Actions.PendingCount(c.Request.Context(), actor.EmployeeID)
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

func (h *Handler) ListRoutes(c *gin.Context) {
	code := StepCode(c.Query("code"))
	if code != "" && !code.Valid() {
		fail(c, http.StatusBadRequest, "kode langkah tidak dikenal")
		return
	}
	list, err := h.Routes.List(c.Request.Context(), code, c.Query("departemen"))
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"routes": list})
}

func (h *Handler) RouteProblems(c *gin.Context) {
	list, err := h.Routes.ListProblems(c.Request.Context())
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"routes": list})
}

func (h *Handler) Readiness(c *gin.Context) {
	dept := strings.TrimSpace(c.Query("departemen"))
	if dept == "" {
		fail(c, http.StatusBadRequest, "departemen wajib diisi")
		return
	}
	plan, err := h.Chains.ResolveChain(c.Request.Context(), dept, "")
	if err != nil {
		failErr(c, err)
		return
	}
	steps := make([]gin.H, 0, len(plan.Steps))
	for _, s := range plan.Steps {
		steps = append(steps, gin.H{
			"code": s.Code, "label": s.Label, "stepOrder": s.StepOrder,
			"status": s.Status, "approvers": s.Approvers,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"departemen": dept,
		"ready":      plan.OK(),
		"problems":   plan.Problems,
		"steps":      steps,
	})
}

func (h *Handler) CreateRoute(c *gin.Context) {
	var in RouteInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "code dan approverEmployeeId wajib diisi")
		return
	}
	in.Departemen = strings.TrimSpace(in.Departemen)
	if msg := in.Validate(); msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	rt, err := h.Routes.Create(c.Request.Context(), in)
	if err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, rt)
}

func (h *Handler) DeleteRoute(c *gin.Context) {
	if err := h.Routes.Delete(c.Request.Context(), c.Param("id")); err != nil {
		failErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Penunjukan dihapus"})
}
