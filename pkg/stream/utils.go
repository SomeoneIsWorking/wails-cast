package stream

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// downloadFile downloads a file with cookies and headers
func (p *RemoteHLSProxy) downloadFile(url string) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
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

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	return resp, nil
}
