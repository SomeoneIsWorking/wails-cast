package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"wails-cast/pkg/extractor"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/sleepinhibit"
)

// RemoteHLSProxy is a proxy server that serves HLS manifests and segments
// with captured cookies and headers
type RemoteHLSProxy struct {
	BaseURL             string
	Manifest            string
	Cookies             map[string]string
	Headers             map[string]string
	LocalIP             string
	Port                int
	CacheDir            string // Directory for caching transcoded segments
	ManifestData        *Manifest
	ManifestPath        string
	httpServer          *http.Server
	AudioPlaylistURL    string // For demuxed HLS: URL of audio playlist
	VideoPlaylistURL    string // For demuxed HLS: URL of video playlist
	IsManifestRewritten bool
	sleepInhibitor      *sleepinhibit.Inhibitor
}

// NewRemoteHLSProxy creates a new HLS proxy server
func NewRemoteHLSProxy(localIP string, port int, cacheDir string) *RemoteHLSProxy {
	// Create cache directory if it doesn't exist
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "hls-proxy-cache")
	}
	os.MkdirAll(cacheDir, 0755)

	manifestPath := filepath.Join(cacheDir, "manifest.json")
	manifestData := &Manifest{
		Items:       make(map[string]*ManifestItem),
		SegmentMap:  make(map[string]string),
		URLMap:      make(map[string]string),
		NextID:      0,
		AudioNextID: 0,
		VideoNextID: 0,
	}

	// Load existing manifest if available
	if data, err := os.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(data, manifestData); err == nil {
			// Check version
			if manifestData.Version != CurrentManifestVersion {
				fmt.Printf("⚠️ Manifest version mismatch (found %d, expected %d). Resetting cache.\n", manifestData.Version, CurrentManifestVersion)
				// Reset manifest
				manifestData = &Manifest{
					Version:     CurrentManifestVersion,
					Items:       make(map[string]*ManifestItem),
					SegmentMap:  make(map[string]string),
					URLMap:      make(map[string]string),
					NextID:      0,
					AudioNextID: 0,
					VideoNextID: 0,
				}
				// Clear cache directory contents (preserve directory itself)
				dirEntries, _ := os.ReadDir(cacheDir)
				for _, entry := range dirEntries {
					os.RemoveAll(filepath.Join(cacheDir, entry.Name()))
				}
			} else {
				fmt.Println("✅ Loaded existing manifest from disk")
			}
		}
	} else {
		// New manifest
		manifestData.Version = CurrentManifestVersion
	}

	return &RemoteHLSProxy{
		LocalIP:        localIP,
		Port:           port,
		CacheDir:       cacheDir,
		ManifestData:   manifestData,
		ManifestPath:   manifestPath,
		sleepInhibitor: sleepinhibit.NewInhibitor(nil), // nil logger for stream package
	}
}

// SetExtractor sets the extractor result for the proxy
func (p *RemoteHLSProxy) SetExtractor(result *extractor.ExtractResult) {
	p.BaseURL = result.BaseURL
	p.Manifest = result.ManifestBody
	p.Cookies = result.Cookies
	p.Headers = result.Headers
}

// Start starts the proxy HTTP server
func (p *RemoteHLSProxy) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/playlist.m3u8", p.handleManifest)
	mux.HandleFunc("/audio.m3u8", p.handleAudioPlaylist)
	mux.HandleFunc("/video.m3u8", p.handleVideoPlaylist)
	mux.HandleFunc("/segment/", p.handleSegment)
	mux.HandleFunc("/debug/log", p.handleDebugLog) // Remote logging from receiver

	// Catch-all for debugging
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("⚠️ Unhandled request: %s\n", r.URL.Path)
		http.NotFound(w, r)
	})

	p.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", p.Port),
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 0, // No write timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("Starting HLS proxy on port %d\n", p.Port)
	go func() {
		if err := p.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Proxy server error: %v\n", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the proxy server
func (p *RemoteHLSProxy) Stop() error {
	// Stop sleep inhibition
	if p.sleepInhibitor != nil {
		p.sleepInhibitor.Stop()
	}

	if p.httpServer != nil {
		return p.httpServer.Close()
	}
	return nil
}

// GetProxyURL returns the URL to cast to Chromecast
func (p *RemoteHLSProxy) GetProxyURL() string {
	return fmt.Sprintf("http://%s:%d/playlist.m3u8", p.LocalIP, p.Port)
}

// GetServedManifest returns the manifest as it would be served
func (p *RemoteHLSProxy) GetServedManifest() string {
	if p.IsManifestRewritten {
		return p.Manifest
	}

	var rewrittenManifest string

	// Detect if this is a master playlist with separate audio/video	// Check if it's a master playlist
	if hls.ParsePlaylistType(p.Manifest) == hls.PlaylistTypeMaster {
		audioTracks, videoTracks := hls.ExtractTracksFromMaster(p.Manifest)

		// Use first available tracks
		if len(audioTracks) > 0 {
			p.AudioPlaylistURL = hls.ResolveURL(p.BaseURL, audioTracks[0].URI)
			p.VideoPlaylistURL = hls.ResolveURL(p.BaseURL, videoTracks[0].URI)

			// Cache the nested playlists
			p.downloadAndParseNestedPlaylists()

			// Rewrite the master playlist to use /audio.m3u8 and /video.m3u8
			rewrittenManifest = p.rewriteDemuxedMaster(p.Manifest, p.BaseURL)
		} else {
			// Not demuxed, proceed with normal rewriting
			rewrittenManifest = p.rewriteManifest(p.Manifest, p.BaseURL, "")
		}
	} else {
		// Not master, proceed with normal rewriting
		rewrittenManifest = p.rewriteManifest(p.Manifest, p.BaseURL, "")
	}

	p.Manifest = rewrittenManifest
	p.IsManifestRewritten = true
	return rewrittenManifest
}

// handleManifest serves the m3u8 manifest with rewritten URLs
func (p *RemoteHLSProxy) handleManifest(w http.ResponseWriter, r *http.Request) {
	// Briefly inhibit sleep on streaming requests (auto-stops after 30s of inactivity)
	if p.sleepInhibitor != nil {
		p.sleepInhibitor.Refresh(30 * time.Second)
	}

	fmt.Printf("Serving manifest to %s\n", r.RemoteAddr)

	limit := 500
	if len(p.Manifest) < limit {
		limit = len(p.Manifest)
	}
	fmt.Printf("Original manifest (first %d chars): %s\n", limit, p.Manifest[:limit])

	rewrittenManifest := p.GetServedManifest()

	limit = 500
	if len(rewrittenManifest) < limit {
		limit = len(rewrittenManifest)
	}
	fmt.Printf("Rewritten main manifest (first %d chars): %s\n", limit, rewrittenManifest[:limit])

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/x-mpegURL")
	w.Header().Set("Cache-Control", "no-cache")

	w.Write([]byte(rewrittenManifest))
}

// handleAudioPlaylist serves the audio playlist
func (p *RemoteHLSProxy) handleAudioPlaylist(w http.ResponseWriter, r *http.Request) {
	audioPath := filepath.Join(p.CacheDir, "audio.m3u8")
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		http.Error(w, "Audio playlist not cached", http.StatusNotFound)
		return
	}

	p.servePlaylistWithPrefix(w, audioPath, p.AudioPlaylistURL, "audio_")
}

// handleVideoPlaylist serves the video playlist
func (p *RemoteHLSProxy) handleVideoPlaylist(w http.ResponseWriter, r *http.Request) {
	videoPath := filepath.Join(p.CacheDir, "video.m3u8")
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "Video playlist not cached", http.StatusNotFound)
		return
	}

	p.servePlaylistWithPrefix(w, videoPath, p.VideoPlaylistURL, "video_")
}

// servePlaylist serves a local playlist file
func (p *RemoteHLSProxy) servePlaylist(w http.ResponseWriter, path string, originalURL string) {
	p.servePlaylistWithPrefix(w, path, originalURL, "")
}

// servePlaylistWithPrefix serves a local playlist file with a specific segment prefix
func (p *RemoteHLSProxy) servePlaylistWithPrefix(w http.ResponseWriter, path string, originalURL string, prefix string) {
	content, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Failed to read playlist", http.StatusInternalServerError)
		return
	}

	// Derive base URL from original URL
	baseURL := originalURL
	if idx := strings.LastIndex(originalURL, "/"); idx != -1 {
		baseURL = originalURL[:idx+1]
	}

	rewritten := p.rewriteManifest(string(content), baseURL, prefix)

	// Ensure proper HLS tags for Chromecast compatibility
	rewritten = p.ensureHLSTags(rewritten)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(rewritten))
}

// downloadAndParseNestedPlaylists downloads audio and video playlists and caches them
func (p *RemoteHLSProxy) downloadAndParseNestedPlaylists() {
	fmt.Println("Downloading nested playlists to cache them...")

	// Download audio playlist
	if p.AudioPlaylistURL != "" {
		resp, err := p.downloadFile(context.Background(), p.AudioPlaylistURL)
		if err != nil {
			fmt.Printf("Failed to download audio playlist: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		audioPlaylist := string(body)

		// Save audio playlist to cache
		audioLocalPath := filepath.Join(p.CacheDir, "audio.m3u8")
		if err := os.WriteFile(audioLocalPath, []byte(audioPlaylist), 0644); err == nil {
			audioItem := &ManifestItem{
				URL:         p.AudioPlaylistURL,
				ContentType: "application/vnd.apple.mpegurl",
				LocalPath:   audioLocalPath,
				IsPlaylist:  true,
			}
			p.updateManifest(p.AudioPlaylistURL, audioItem)
		}

		fmt.Printf("Cached audio playlist as audio.m3u8\n")
	}

	// Download video playlist
	if p.VideoPlaylistURL != "" {
		resp, err := p.downloadFile(context.Background(), p.VideoPlaylistURL)
		if err != nil {
			fmt.Printf("Failed to download video playlist: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		videoPlaylist := string(body)

		// Save video playlist to cache
		videoLocalPath := filepath.Join(p.CacheDir, "video.m3u8")
		if err := os.WriteFile(videoLocalPath, []byte(videoPlaylist), 0644); err == nil {
			videoItem := &ManifestItem{
				URL:         p.VideoPlaylistURL,
				ContentType: "application/vnd.apple.mpegurl",
				LocalPath:   videoLocalPath,
				IsPlaylist:  true,
			}
			p.updateManifest(p.VideoPlaylistURL, videoItem)
		}

		fmt.Printf("Cached video playlist as video.m3u8\n")
	}
}

// rewriteManifest rewrites URLs in the manifest to point to the proxy
func (p *RemoteHLSProxy) rewriteManifest(manifest string, baseURL string, segmentPrefix string) string {
	isMaster := strings.Contains(manifest, "#EXT-X-STREAM-INF")
	lines := strings.Split(manifest, "\n")

	base, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("Error parsing base URL %s: %v\n", baseURL, err)
		return manifest
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip if already rewritten
		if strings.Contains(trimmed, p.LocalIP) || strings.Contains(trimmed, "/segment/") || strings.Contains(trimmed, "/playlist/") || strings.Contains(trimmed, "/audio.m3u8") || strings.Contains(trimmed, "/video.m3u8") {
			continue
		}

		// Check for URI="..." pattern (e.g. for keys or subtitles)
		if strings.Contains(line, `URI="`) {
			uriPattern := regexp.MustCompile(`URI="([^"]+)"`)
			line = uriPattern.ReplaceAllStringFunc(line, func(match string) string {
				path := uriPattern.FindStringSubmatch(match)[1]

				// Resolve to absolute URL
				u, err := url.Parse(path)
				if err == nil {
					path = base.ResolveReference(u).String()
				}

				id := p.getOrAssignID(path, "")

				if isMaster {
					return fmt.Sprintf(`URI="/playlist/%s.m3u8"`, id)
				}

				// Determine extension based on path or default to .ts
				ext := filepath.Ext(path)
				if ext == "" {
					ext = ".ts"
				}
				return fmt.Sprintf(`URI="/segment/%s%s%s"`, segmentPrefix, id, ext)
			})
			lines[i] = line
			continue
		}

		// Check for segment URLs (lines not starting with #)
		if !strings.HasPrefix(trimmed, "#") {
			// Resolve to absolute URL
			u, err := url.Parse(trimmed)
			if err == nil {
				trimmed = base.ResolveReference(u).String()
			}

			id := p.getOrAssignID(trimmed, segmentPrefix)
			lines[i] = fmt.Sprintf("/segment/%s%s.ts", segmentPrefix, id)
		}
	}

	return strings.Join(lines, "\n")
}

// rewriteDemuxedMaster rewrites a demuxed master playlist to use /audio.m3u8 and /video.m3u8
func (p *RemoteHLSProxy) rewriteDemuxedMaster(manifest string, baseURL string) string {
	lines := strings.Split(manifest, "\n")
	var result []string
	expectingVideoURI := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}

		// Skip if already rewritten
		if strings.Contains(trimmed, p.LocalIP) || strings.Contains(trimmed, "/segment/") || strings.Contains(trimmed, "/playlist/") || strings.Contains(trimmed, "/audio.m3u8") || strings.Contains(trimmed, "/video.m3u8") {
			result = append(result, line)
			expectingVideoURI = false
			continue
		}

		// Check for URI="..." in EXT-X-MEDIA TYPE=AUDIO
		if strings.Contains(line, `TYPE=AUDIO`) && strings.Contains(line, `URI="`) {
			// Replace URI with /audio.m3u8
			uriPattern := regexp.MustCompile(`URI="([^"]+)"`)
			line = uriPattern.ReplaceAllString(line, `URI="/audio.m3u8"`)
			result = append(result, line)
			expectingVideoURI = false
			continue
		}

		// If expecting video URI and this is not a comment/tag, replace with /video.m3u8
		if expectingVideoURI && !strings.HasPrefix(trimmed, "#") {
			result = append(result, "/video.m3u8")
			expectingVideoURI = false
			continue
		}

		// Check for #EXT-X-STREAM-INF
		if strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF") {
			expectingVideoURI = true
		} else {
			expectingVideoURI = false
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// ensureHLSTags adds required HLS tags if missing for better Chromecast compatibility
func (p *RemoteHLSProxy) ensureHLSTags(playlist string) string {
	lines := strings.Split(playlist, "\n")
	var result []string

	hasVersion := false
	hasTargetDuration := false
	hasMediaSequence := false
	isMediaPlaylist := false

	// First pass: check what we have
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-VERSION") {
			hasVersion = true
		} else if strings.HasPrefix(trimmed, "#EXT-X-TARGETDURATION") {
			hasTargetDuration = true
		} else if strings.HasPrefix(trimmed, "#EXT-X-MEDIA-SEQUENCE") {
			hasMediaSequence = true
		} else if strings.HasPrefix(trimmed, "#EXTINF") {
			isMediaPlaylist = true
		}
	}

	// If it's not a media playlist (it's a master playlist), don't add these tags
	if !isMediaPlaylist {
		return playlist
	}

	// Second pass: add missing tags
	inHeader := true
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Add to result
		result = append(result, line)

		// After #EXTM3U, add missing header tags
		if inHeader && strings.HasPrefix(trimmed, "#EXTM3U") {
			if !hasVersion {
				result = append(result, "#EXT-X-VERSION:3")
			}
			if !hasTargetDuration && isMediaPlaylist {
				result = append(result, "#EXT-X-TARGETDURATION:10")
			}
			if !hasMediaSequence && isMediaPlaylist {
				result = append(result, "#EXT-X-MEDIA-SEQUENCE:0")
			}
			inHeader = false
		}

		// Stop adding header tags after first segment
		if i > 0 && !strings.HasPrefix(trimmed, "#") && trimmed != "" {
			inHeader = false
		}
	}

	return strings.Join(result, "\n")
}
