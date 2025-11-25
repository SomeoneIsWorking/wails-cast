package extractor

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// ExtractResult contains the extracted video information
type ExtractResult struct {
	URL          string            // Original HLS URL
	BaseURL      string            // Base URL (scheme + host) for resolving relative paths
	ManifestPath string            // Path to saved m3u8 file
	ManifestBody string            // Raw m3u8 content
	Cookies      map[string]string // Captured cookies
	Headers      map[string]string // Captured headers
}

// ExtractVideo opens a browser, navigates to the URL, and extracts the HLS stream
// by intercepting network requests and checking Content-Type headers.
// It captures cookies, headers, and saves the m3u8 manifest.
func ExtractVideo(pageURL string) (*ExtractResult, error) {
	// Launch system browser (to ensure codecs) in headful mode
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).Headless(false).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	// Create a new page
	page := browser.MustPage("")

	// Channel to receive the found video
	foundVideo := make(chan *ExtractResult, 1)

	// Setup request hijacking
	router := page.HijackRequests()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		// Load the response to check headers
		err := ctx.LoadResponse(http.DefaultClient, true)
		if err != nil {
			// If loading fails, just continue
			return
		}

		// Check Content-Type header
		contentType := ctx.Response.Headers().Get("Content-Type")
		if strings.Contains(contentType, "application/vnd.apple.mpegurl") ||
			strings.Contains(contentType, "application/x-mpegURL") {
			reqURL := ctx.Request.URL().String()
			fmt.Printf("Found HLS stream: %s (Content-Type: %s)\n", reqURL, contentType)

			// Parse base URL
			parsedURL, err := url.Parse(reqURL)
			if err != nil {
				fmt.Printf("Failed to parse URL: %v\n", err)
				return
			}
			baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
			fmt.Printf("Base URL: %s\n", baseURL)

			// Get the response body (m3u8 manifest) - it's already a string in Rod
			manifestContent := ctx.Response.Body()
			fmt.Printf("Manifest content length: %d bytes\n", len(manifestContent))

			// Capture cookies from request
			cookies := make(map[string]string)
			cookieHeader := ctx.Request.Header("Cookie")
			if cookieHeader != "" {
				// Parse cookie header
				for _, cookie := range strings.Split(cookieHeader, "; ") {
					parts := strings.SplitN(cookie, "=", 2)
					if len(parts) == 2 {
						cookies[parts[0]] = parts[1]
					}
				}
			}
			fmt.Printf("Captured %d cookies\n", len(cookies))

			// Capture important headers
			headers := make(map[string]string)
			importantHeaders := []string{"User-Agent", "Referer", "Origin", "Accept", "Accept-Language"}
			for _, headerName := range importantHeaders {
				if value := ctx.Request.Header(headerName); value != "" {
					headers[headerName] = value
				}
			}
			fmt.Printf("Captured %d headers\n", len(headers))

			// Save manifest to temp file (for debugging)
			tmpDir := os.TempDir()
			manifestPath := filepath.Join(tmpDir, fmt.Sprintf("extracted_stream_%d.m3u8", time.Now().Unix()))

			err = os.WriteFile(manifestPath, []byte(manifestContent), 0644)
			if err != nil {
				fmt.Printf("Failed to save manifest: %v\n", err)
				return
			}

			fmt.Printf("Saved manifest to: %s\n", manifestPath)

			result := &ExtractResult{
				URL:          reqURL,
				BaseURL:      baseURL,
				ManifestPath: manifestPath,
				ManifestBody: manifestContent,
				Cookies:      cookies,
				Headers:      headers,
			}

			select {
			case foundVideo <- result:
			default:
			}
		}
	})

	// Start the router
	go router.Run()

	// Set User Agent to Safari to encourage HLS
	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
	})

	fmt.Printf("Navigating to %s...\n", pageURL)
	page.MustNavigate(pageURL)
	page.MustWaitLoad()

	// Try to auto-click the play button using JavaScript (bypasses pointer-events)
	fmt.Println("Looking for play button...")
	var err error
	_, err = page.Eval(`() => {
		var playBtn = document.querySelector('#play-video');
		if (playBtn) {
			$(playBtn).parent().find("*").trigger("click");
		}
	}`)
	if err != nil {
		fmt.Printf("Failed to click play button: %v\n", err)
	} else {
		fmt.Println("Clicked play button via JavaScript")
		time.Sleep(2 * time.Second) // Wait for player to load
	}

	fmt.Println("Waiting for video URL... Please play the video if it doesn't start automatically.")

	// Wait for URL or timeout
	select {
	case result := <-foundVideo:
		return result, nil
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timeout waiting for video URL")
	}
}
