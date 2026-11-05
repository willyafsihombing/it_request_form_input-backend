package handlers

import (
	"context"
	"fmt"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

type PDFHandler struct {
	Repo *repository.RequestRepository
}

func NewPDFHandler(repo *repository.RequestRepository) *PDFHandler {
	return &PDFHandler{Repo: repo}
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

	// now := time.Now()
	// monthYear := now.Format("012006")
	fileName := fmt.Sprintf("%s.pdf", req.FormNumber) //->%s_%s

	printURL := fmt.Sprintf("http://localhost:3030/dashboard/requests/%s/print", id)

	pdfBuf, err := generatePDFFromURL(printURL)
	if err != nil {
		utils.InternalError(c, "Gagal generate PDF: "+err.Error())
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Length", fmt.Sprintf("%d", len(pdfBuf)))

	c.Data(200, "application/pdf", pdfBuf)

}

func generatePDFFromURL(url string) ([]byte, error) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var pdfBuf []byte

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),

		chromedp.WaitVisible(`#print-area`, chromedp.ByID),

		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithLandscape(true).
				WithPaperWidth(11.69).
				WithPaperHeight(8.27).
				WithMarginTop(0.16).
				WithMarginBottom(0.16).
				WithMarginLeft(0.16).
				WithMarginRight(0.16).
				WithScale(0.9). // scale 90% agar muat
				Do(ctx)
			return err
		}),
	)

	return pdfBuf, err
}
