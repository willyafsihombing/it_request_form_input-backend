package handlers

import (
	"context"
	"fmt"
	"it-request-backend/repository"
	"it-request-backend/utils"
	"os"
	"strings"
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

func frontendBase() string {
	if v := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/"); v != "" {
		return v
	}
	return "http://localhost:3000"
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

	fileName := fmt.Sprintf("%s.pdf", req.FormNumber) //->%s_%s

	printURL := fmt.Sprintf("%s/dashboard/requests/%s/print", frontendBase(), id)

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
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

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
				WithScale(0.9).
				Do(ctx)
			return err
		}),
	)

	return pdfBuf, err
}

// package handlers

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	neturl "net/url"
// 	"os"
// 	"strings"
// 	"time"

// 	"it-request-backend/repository"
// 	"it-request-backend/utils"

// 	"github.com/chromedp/cdproto/network"
// 	"github.com/chromedp/cdproto/page"
// 	"github.com/chromedp/chromedp"
// 	"github.com/gin-gonic/gin"
// )

// type PDFHandler struct {
// 	Repo *repository.RequestRepository
// }

// func NewPDFHandler(repo *repository.RequestRepository) *PDFHandler {
// 	return &PDFHandler{Repo: repo}
// }

// func frontendBase() string {
// 	if v := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/"); v != "" {
// 		return v
// 	}
// 	return "http://127.0.0.1:3030"
// }

// // GET /api/requests/:id/pdf
// func (h *PDFHandler) GeneratePDF(c *gin.Context) {
// 	id := c.Param("id")

// 	req, err := h.Repo.FindByID(id)
// 	if err != nil {
// 		utils.InternalError(c, "Internal Server Error")
// 		return
// 	}
// 	if req == nil {
// 		utils.NotFound(c, "Request not found")
// 		return
// 	}

// 	fileName := fmt.Sprintf("%s.pdf", req.FormNumber)
// 	printURL := fmt.Sprintf("%s/dashboard/requests/%s/print", frontendBase(), id)

// 	token, _ := c.Cookie("auth-token")

// 	pdfBuf, err := generatePDFFromURL(printURL, token)
// 	if err != nil {
// 		log.Println("generate PDF error:", err)
// 		utils.InternalError(c, "Gagal generate PDF: "+err.Error())
// 		return
// 	}

// 	c.Header("Content-Type", "application/pdf")
// 	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
// 	c.Header("Content-Length", fmt.Sprintf("%d", len(pdfBuf)))
// 	c.Data(200, "application/pdf", pdfBuf)
// }

// func generatePDFFromURL(url, token string) ([]byte, error) {
// 	opts := append(chromedp.DefaultExecAllocatorOptions[:],
// 		chromedp.NoSandbox,
// 		chromedp.Flag("disable-gpu", true),
// 		chromedp.Flag("disable-dev-shm-usage", true),
// 		chromedp.Flag("no-proxy-server", true),
// 	)
// 	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
// 	defer cancelAlloc()

// 	ctx, cancel := chromedp.NewContext(allocCtx)
// 	defer cancel()

// 	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
// 	defer cancelTimeout()

// 	u, _ := neturl.Parse(url)

// 	setup := chromedp.Tasks{
// 		chromedp.ActionFunc(func(ctx context.Context) error {
// 			if token == "" {
// 				return nil
// 			}
// 			return network.SetCookie("auth-token", token).
// 				WithDomain(u.Hostname()).
// 				WithPath("/").
// 				Do(ctx)
// 		}),
// 		chromedp.Navigate(url),
// 	}
// 	if err := chromedp.Run(ctx, setup); err != nil {
// 		return nil, fmt.Errorf("gagal membuka %s: %w", url, err)
// 	}

// 	var loc string
// 	_ = chromedp.Run(ctx, chromedp.Location(&loc))

// 	var pdfBuf []byte
// 	err := chromedp.Run(ctx,
// 		chromedp.WaitVisible(`#print-area`, chromedp.ByID),
// 		chromedp.ActionFunc(func(ctx context.Context) error {
// 			var err error
// 			pdfBuf, _, err = page.PrintToPDF().
// 				WithPrintBackground(true).
// 				WithLandscape(true).
// 				WithPaperWidth(11.69).
// 				WithPaperHeight(8.27).
// 				WithMarginTop(0.16).
// 				WithMarginBottom(0.16).
// 				WithMarginLeft(0.16).
// 				WithMarginRight(0.16).
// 				WithScale(0.9).
// 				Do(ctx)
// 			return err
// 		}),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("halaman berakhir di %s: %w", loc, err)
// 	}
// 	return pdfBuf, nil
// }
