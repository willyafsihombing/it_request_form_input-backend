package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

var client = &http.Client{Timeout: 60 * time.Second}

func serviceURL() string {
	if v := strings.TrimRight(os.Getenv("PDF_SERVICE_URL"), "/"); v != "" {
		return v
	}
	return "http://localhost:3001"
}

func URLToPDF(ctx context.Context, pageURL string, fields map[string]string) ([]byte, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	if err := w.WriteField("url", pageURL); err != nil {
		return nil, err
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		serviceURL()+"/forms/chromium/convert/url", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pdf service tidak bisa dihubungi: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pdf service error %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
