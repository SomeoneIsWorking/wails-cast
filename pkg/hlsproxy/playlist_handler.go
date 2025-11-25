package hlsproxy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleManifest serves the m3u8 manifest with rewritten URLs
func (p *HLSProxy) handleManifest(w http.ResponseWriter, r *http.Request) {
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
func (p *HLSProxy) handleAudioPlaylist(w http.ResponseWriter, r *http.Request) {
	audioPath := filepath.Join(p.CacheDir, "audio.m3u8")
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		http.Error(w, "Audio playlist not cached", http.StatusNotFound)
		return
	}

	p.servePlaylistWithPrefix(w, audioPath, p.AudioPlaylistURL, "audio_")
}

// handleVideoPlaylist serves the video playlist
func (p *HLSProxy) handleVideoPlaylist(w http.ResponseWriter, r *http.Request) {
	videoPath := filepath.Join(p.CacheDir, "video.m3u8")
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		http.Error(w, "Video playlist not cached", http.StatusNotFound)
		return
	}

	p.servePlaylistWithPrefix(w, videoPath, p.VideoPlaylistURL, "video_")
}

// servePlaylist serves a local playlist file
func (p *HLSProxy) servePlaylist(w http.ResponseWriter, path string, originalURL string) {
	p.servePlaylistWithPrefix(w, path, originalURL, "")
}

// servePlaylistWithPrefix serves a local playlist file with a specific segment prefix
func (p *HLSProxy) servePlaylistWithPrefix(w http.ResponseWriter, path string, originalURL string, prefix string) {
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
