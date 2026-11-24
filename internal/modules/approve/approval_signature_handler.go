package approve

import (
	"it-request-backend/internal/shared/utils"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SignatureHandler struct {
	Repo *ActionRepository
}

func NewSignatureHandler(repo *ActionRepository) *SignatureHandler {
	return &SignatureHandler{Repo: repo}
}

func (h *SignatureHandler) userID(c *gin.Context) (string, bool) {
	cl := utils.CurrentClaims(c)
	if cl == nil || cl.ID == "" {
		utils.Forbidden(c, "Akun tidak valid")
		return "", false
	}
	return cl.ID, true
}

// GET /api/me/signature
func (h *SignatureHandler) Get(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		return
	}
	img, err := h.Repo.GetSignature(c.Request.Context(), uid)
	if err != nil {
		utils.InternalError(c, "Gagal membaca tanda tangan")
		return
	}
	utils.OK(c, SignatureResponse{Image: img, Has: img != ""})
}

// PUT /api/me/signature
func (h *SignatureHandler) Put(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sigMaxBody+1024)

	var in SignatureInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "Body tidak valid atau terlalu besar")
		return
	}
	if err := in.Validate(); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := h.Repo.SaveSignature(c.Request.Context(), uid, in.Image); err != nil {
		log.Printf("[signature] save error: %v", err)
		utils.InternalError(c, "Gagal menyimpan tanda tangan")
		return
	}
	utils.OK(c, gin.H{"saved": true})
}

func (h *SignatureHandler) SignAfter(c *gin.Context) {
	c.Next()
	if c.Writer.Status() >= 300 {
		return
	}
	ctx := c.Request.Context()

	if id := c.Param("id"); id != "" {
		if err := h.Repo.AttachSignatures(ctx, id); err != nil {
			log.Printf("[signature] attach steps error %v", err)
		}
	}
	if cl := utils.CurrentClaims(c); cl != nil && cl.ID != "" {
		if err := h.Repo.AttachRequesterSignature(ctx, cl.ID); err != nil {
			log.Printf("[signature] attach requester error : %v", err)
		}
	}
}

// DELETE /api/me/signature
func (h *SignatureHandler) Delete(c *gin.Context) {
	uid, ok := h.userID(c)
	if !ok {
		return
	}
	if err := h.Repo.DeleteSignature(c.Request.Context(), uid); err != nil {
		utils.InternalError(c, "Gagal menghapus tanda tangan")
		return
	}
	utils.OK(c, gin.H{"deleted": true})
}

// GET /api/requests/:id/signatures
func (h *SignatureHandler) ForRequest(c *gin.Context) {
	sigs, err := h.Repo.SignaturesFor(c.Request.Context(), c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Gagal membaca tanda tangan")
		return
	}
	utils.OK(c, gin.H{"signatures": sigs})
}

func (h *SignatureHandler) ForPrint(c *gin.Context) {
	id := c.Param("id")
	if !utils.VerifyPrintToken(id, c.Query("pt")) {
		utils.Forbidden(c, "Token cetak tidak valid")
		return
	}
	sigs, err := h.Repo.SignaturesFor(c.Request.Context(), id)
	if err != nil {
		utils.InternalError(c, "Gagal membaca tanda tangan")
		return
	}
	utils.OK(c, gin.H{"signatures": sigs})
}
