package hlsproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"wails-cast/pkg/extractor"
)

// HLSProxy is a proxy server that serves HLS manifests and segments
// with captured cookies and headers
type HLSProxy struct {
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
}

// NewHLSProxy creates a new HLS proxy server
func NewHLSProxy(result *extractor.ExtractResult, localIP string, port int, cacheDir string) *HLSProxy {
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
		json.Unmarshal(data, manifestData)
	}

	return &HLSProxy{
		BaseURL:      result.BaseURL,
		Manifest:     result.ManifestBody,
		Cookies:      result.Cookies,
		Headers:      result.Headers,
		LocalIP:      localIP,
		Port:         port,
		CacheDir:     cacheDir,
		ManifestData: manifestData,
		ManifestPath: manifestPath,
	}
}

// Start starts the proxy HTTP server
func (p *HLSProxy) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/playlist.m3u8", p.handleManifest)
	mux.HandleFunc("/audio.m3u8", p.handleAudioPlaylist)
	mux.HandleFunc("/video.m3u8", p.handleVideoPlaylist)
	mux.HandleFunc("/playlist/", p.handleSegment)
	mux.HandleFunc("/segment/", p.handleSegment)

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
func (p *HLSProxy) Stop() error {
	if p.httpServer != nil {
		return p.httpServer.Close()
	}
	return nil
}

// GetProxyURL returns the URL to cast to Chromecast
func (p *HLSProxy) GetProxyURL() string {
	return fmt.Sprintf("http://%s:%d/playlist.m3u8", p.LocalIP, p.Port)
}

// GetServedManifest returns the manifest as it would be served
func (p *HLSProxy) GetServedManifest() string {
	if p.IsManifestRewritten {
		return p.Manifest
	}

	var rewrittenManifest string

	// Detect if this is a master playlist with separate audio/video tracks (demuxed HLS)
	if ParsePlaylistType(p.Manifest) == PlaylistTypeMaster {
		audioTracks, videoTracks := ExtractTracksFromMaster(p.Manifest)

		// Use the first audio and video track URLs
		if len(audioTracks) > 0 && len(videoTracks) > 0 {
			// Resolve URLs
			p.AudioPlaylistURL = ResolveURL(p.BaseURL, audioTracks[0].URI)
			p.VideoPlaylistURL = ResolveURL(p.BaseURL, videoTracks[0].URI)

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
