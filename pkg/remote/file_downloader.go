package remote

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FileDownloader struct {
	Cookies map[string]string
	Headers map[string]string
}

// DownloadFile downloads a file with cookies and headers
func (p *FileDownloader) DownloadFile(ctx context.Context, url *url.URL) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, err
	}

	for key, value := range p.Headers {
		req.Header.Set(key, value)
	}

	if len(p.Cookies) > 0 {
		var cookieParts []string
		for key, value := range p.Cookies {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", key, value))
		}
		req.Header.Set("Cookie", strings.Join(cookieParts, "; "))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
