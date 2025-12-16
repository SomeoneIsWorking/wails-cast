package extractor

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	// WaitAfterManifestFound is the time to wait after finding m3u8 to capture subtitles
	WaitAfterManifestFound = 2 * time.Second
)

// ExtractedSubtitleTrack represents a captured subtitle file
type ExtractedSubtitleTrack struct {
	URL     string // Subtitle URL
	Content string `json:"-"` // Raw VTT content
	Charset string // Charset from Content-Type (e.g., "UTF-8", "Windows-1254")
	Label   string // Optional label (extracted from URL or filename)
}

// ExtractResult contains the extracted video information
type ExtractResult struct {
	SiteURL     string                   // Original page URL
	URL         string                   // Original HLS URL
	Title       string                   // Optional title
	BaseURL     string                   // Base URL (scheme + host) for resolving relative paths
	ManifestRaw string                   `json:"-"` // Raw m3u8 content
	Cookies     map[string]string        // Captured cookies
	Headers     map[string]string        // Captured headers
	Subtitles   []ExtractedSubtitleTrack // Captured subtitle tracks
}

type PlaylistExtractionResult struct {
	URL         string            // Original HLS URL
	BaseURL     string            // Base URL (scheme + host) for resolving relative paths
	Cookies     map[string]string // Captured cookies
	Headers     map[string]string // Captured headers
	ManifestRaw string            // Raw m3u8 content
}

// handlePlaylist processes an HLS manifest request
func handlePlaylist(ctx *rod.Hijack) *PlaylistExtractionResult {
	reqURL := ctx.Request.URL().String()
	contentType := ctx.Response.Headers().Get("Content-Type")

	fmt.Printf("Found HLS stream: %s (Content-Type: %s)\n", reqURL, contentType)

	// Parse base URL
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		fmt.Printf("Failed to parse URL: %v\n", err)
		return nil
	}
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	fmt.Printf("Base URL: %s\n", baseURL)

	// Get the response body (m3u8 manifest)
	manifestContent := ctx.Response.Body()
	fmt.Printf("Manifest content length: %d bytes\n", len(manifestContent))

	// Capture cookies from request
	cookies := make(map[string]string)
	cookieHeader := ctx.Request.Header("Cookie")
	if cookieHeader != "" {
		// Parse cookie header
		for cookie := range strings.SplitSeq(cookieHeader, "; ") {
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

	fmt.Printf("Manifest found, waiting %v for subtitles...\n", WaitAfterManifestFound)

	return &PlaylistExtractionResult{
		URL:         reqURL,
		BaseURL:     baseURL,
		ManifestRaw: manifestContent,
		Cookies:     cookies,
		Headers:     headers,
	}
}

// handleSubtitle processes a VTT subtitle request
func handleSubtitle(ctx *rod.Hijack, subtitleURLs map[string]bool) *ExtractedSubtitleTrack {
	reqURL := ctx.Request.URL().String()
	contentType := ctx.Response.Headers().Get("Content-Type")

	// Skip if already processed
	if subtitleURLs[reqURL] {
		return nil
	}
	subtitleURLs[reqURL] = true

	fmt.Printf("Found subtitle: %s (Content-Type: %s)\n", reqURL, contentType)

	// Extract charset from Content-Type
	charset := "UTF-8" // default
	if strings.Contains(contentType, "charset=") {
		parts := strings.Split(contentType, "charset=")
		if len(parts) > 1 {
			charset = strings.TrimSpace(strings.Split(parts[1], ";")[0])
		}
	}
	fmt.Printf("Subtitle charset: %s\n", charset)

	// Get subtitle content
	subtitleContent := ctx.Response.Body()
	fmt.Printf("Subtitle content length: %d bytes\n", len(subtitleContent))

	// Extract label from URL (filename without extension)
	label := ""
	if parsedURL, err := url.Parse(reqURL); err == nil {
		pathParts := strings.Split(parsedURL.Path, "/")
		if len(pathParts) > 0 {
			filename := pathParts[len(pathParts)-1]
			label = strings.TrimSuffix(filename, ".vtt")
		}
	}

	return &ExtractedSubtitleTrack{
		URL:     reqURL,
		Content: subtitleContent,
		Charset: charset,
		Label:   label,
	}
}

// ExtractManifestPlaylist opens a browser, navigates to the URL, and extracts the HLS stream
// by intercepting network requests and checking Content-Type headers.
// It captures cookies, headers, and saves the m3u8 manifest.
func ExtractManifestPlaylist(pageURL string) (*ExtractResult, error) {
	// Launch system browser (to ensure codecs) in headful mode
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).Headless(false).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	// Create a new page
	page := browser.MustPage("")

	// Shared result that will accumulate data
	var result = &ExtractResult{
		SiteURL: pageURL,
	}
	var manifestFound bool
	var manifestFoundTime time.Time
	subtitleURLs := make(map[string]bool) // Track processed subtitle URLs

	// Channel to signal completion
	done := make(chan struct{})

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

		if !manifestFound {
			// Check for HLS manifest
			if strings.Contains(contentType, "application/vnd.apple.mpegurl") ||
				strings.Contains(contentType, "application/x-mpegURL") {
				if r := handlePlaylist(ctx); r != nil {
					result.BaseURL = r.BaseURL
					result.Cookies = r.Cookies
					result.Headers = r.Headers
					result.URL = r.URL
					result.ManifestRaw = r.ManifestRaw
					manifestFound = true
					manifestFoundTime = time.Now()
				}
			}
		}

		// Check for VTT subtitles
		if strings.Contains(strings.ToLower(contentType), "text/vtt") {
			if subtitle := handleSubtitle(ctx, subtitleURLs); subtitle != nil {
				result.Subtitles = append(result.Subtitles, *subtitle)
				fmt.Printf("Captured subtitle '%s' (%d total subtitles)\n", subtitle.Label, len(result.Subtitles))
			}
		}
	})

	// Start the router
	go router.Run()

	// Goroutine to handle timing logic
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			if manifestFound && time.Since(manifestFoundTime) >= WaitAfterManifestFound {
				close(done)
				return
			}
		}
	}()

	// Set User Agent to Safari to encourage HLS
	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
	})

	// Override media capability detection APIs to trick JW Player into thinking
	// the browser supports all codecs, even though Chromium lacks H.264/AAC.
	// This prevents JW Player error 102630 (empty playlist due to codec filtering).
	page.MustEvalOnNewDocument(`
		// Override HTMLMediaElement.canPlayType to always return "probably"
		const originalCanPlayType = HTMLMediaElement.prototype.canPlayType;
		HTMLMediaElement.prototype.canPlayType = function(type) {
			console.log('[Codec Override] canPlayType called with:', type);
			// Always return "probably" for any video/audio type
			if (type && (type.includes('video') || type.includes('audio'))) {
				return 'probably';
			}
			return originalCanPlayType.call(this, type);
		};

		// Override MediaSource.isTypeSupported to always return true
		if (window.MediaSource && MediaSource.isTypeSupported) {
			const originalIsTypeSupported = MediaSource.isTypeSupported;
			MediaSource.isTypeSupported = function(type) {
				console.log('[Codec Override] MediaSource.isTypeSupported called with:', type);
				// Always return true for any codec type
				if (type && (type.includes('video') || type.includes('audio'))) {
					return true;
				}
				return originalIsTypeSupported.call(this, type);
			};
		}

		console.log('[Codec Override] Media capability detection overrides installed');
	`)

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

	fmt.Println("Waiting for video URL and subtitles... Please play the video if it doesn't start automatically.")

	// Wait for completion or timeout
	select {
	case <-done:
	case <-time.After(5 * time.Minute):
	}

	if !manifestFound {
		return nil, fmt.Errorf("timeout waiting for video URL")
	}

	result.Title = page.MustInfo().Title
	return result, nil
}
