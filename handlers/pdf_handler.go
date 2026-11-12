package handlers

import (
	"context"
	"fmt"
	"it-request-backend/pdf"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PDFHandler struct {
	Repo *repository.RequestRepository
}

func NewPDFHandler(repo *repository.RequestRepository) *PDFHandler {
	return &PDFHandler{Repo: repo}
}

var pdfSlots = make(chan struct{}, 2)

func printBase() string {
	if v := strings.TrimRight(os.Getenv("PRINT_BASE_URL"), "/"); v != "" {
		return v
	}
	return "http://localhost:3000"
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeFileName(name string) string {
	name = strings.Trim(unsafeFileChars.ReplaceAllString(name, "_"), "_")
	if name == "" {
		name = "IT-Request"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name + ".pdf"
}

var printOptions = map[string]string{
	"landscape":         "true",
	"paperWidth":        "11.69",
	"paperHeight":       "8.27",
	"marginTop":         "0",
	"marginBottom":      "0",
	"marginLeft":        "0",
	"marginRight":       "0",
	"printBackground":   "true",
	"preferCssPageSize": "true",
	"scale":             "1",
	"waitForExpression": "!!document.querySelector('#print-area') && document.fonts.status === 'loaded'",
}

// GET /api/requests/:id/pdf
func (h *PDFHandler) GeneratePDF(c *gin.Context) {
	id := c.Param("id")

	req, err := h.Repo.FindByID(id)
	if err != nil {
		utils.InternalError(c, "Internal Server Error")
		return
	}
	if req == nil {
		utils.NotFound(c, "Request not found")
		return
	}

	select {
	case pdfSlots <- struct{}{}:
		defer func() { <-pdfSlots }()
	case <-c.Request.Context().Done():
		return
	}

	fileName := safeFileName(req.FormNumber)
	printURL := fmt.Sprintf("%s/dashboard/requests/%s/print", printBase(), id)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	pdfBuf, err := pdf.URLToPDF(ctx, printURL, printOptions)
	if err != nil {
		log.Printf("[PDF] gagal untuk %s: %v", id, err)
		utils.InternalError(c, "Gagal membuat PDF, coba lagi")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Data(200, "application/pdf", pdfBuf)
}
