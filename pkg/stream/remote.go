package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"wails-cast/pkg/extractor"
	"wails-cast/pkg/hls"
	_inhibitor "wails-cast/pkg/inhibitor"
	"wails-cast/pkg/logger"
)

var inhibitor = _inhibitor.InhibitorInstance

// RemoteHandler is a handler that serves HLS manifests and segments
// with captured cookies and headers
type RemoteHandler struct {
	BaseURL             string
	Manifest            string
	Cookies             map[string]string
	Headers             map[string]string
	LocalIP             string
	CacheDir            string // Directory for caching transcoded segments
	ManifestData        *Manifest
	ManifestPath        string
	AudioPlaylistURL    string // For demuxed HLS: URL of audio playlist
	VideoPlaylistURL    string // For demuxed HLS: URL of video playlist
	IsManifestRewritten bool
	Options             StreamOptions
	Duration            float64 // Total duration of the stream in seconds
	OriginalManifest    string  // Original manifest content before rewriting
}

// NewRemoteHandler creates a new HLS handler
func NewRemoteHandler(localIP string, cacheDir string, options StreamOptions) *RemoteHandler {
	// Create cache directory if it doesn't exist
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "hls-proxy-cache")
	}
	os.MkdirAll(cacheDir, 0755)

	manifestPath := filepath.Join(cacheDir, "manifest.json")
	manifestData := &Manifest{
		Items: make(map[string]*ManifestItem),
	}

	// Load existing manifest if available
	if data, err := os.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(data, manifestData); err == nil {
			// Check version
			if manifestData.Version != CurrentManifestVersion {
				fmt.Printf("⚠️ Manifest version mismatch (found %d, expected %d). Resetting cache.\n", manifestData.Version, CurrentManifestVersion)
				// Reset manifest
				manifestData = &Manifest{
					Version: CurrentManifestVersion,
					Items:   make(map[string]*ManifestItem),
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

	return &RemoteHandler{
		LocalIP:      localIP,
		CacheDir:     cacheDir,
		ManifestData: manifestData,
		ManifestPath: manifestPath,
		Options:      options,
	}
}

// SetExtractor sets the extractor result for the proxy
func (p *RemoteHandler) SetExtractor(result *extractor.ExtractResult) {
	p.BaseURL = result.BaseURL
	p.Manifest = result.ManifestBody
	p.OriginalManifest = result.ManifestBody
	p.Cookies = result.Cookies
	p.Headers = result.Headers
}

// Cleanup cleans up resources
func (p *RemoteHandler) Cleanup() {
	// Stop sleep inhibition
	inhibitor.Stop()
}

// GetServedManifest returns the manifest as it would be served
func (p *RemoteHandler) GetServedManifest() string {
	if p.IsManifestRewritten {
		return p.Manifest
	}

	var rewrittenManifest string

	// Check if it's a master playlist
	if hls.ParsePlaylistType(p.Manifest) == hls.PlaylistTypeMaster {
		// Rewrite the master playlist to use /audio_{i}.m3u8 and /video_{i}.m3u8
		rewrittenManifest = p.rewriteMasterPlaylist(p.Manifest)
		// Save rewritten master
		// Ensure directory exists (cacheMasterPlaylist should have created it, but GetServedManifest might be called first?)
		// Actually serveMasterPlaylist calls cacheMasterPlaylist first.
		// But let's be safe.
		dirPath := filepath.Join(p.CacheDir, "playlist")
		os.MkdirAll(dirPath, 0755)
		os.WriteFile(filepath.Join(dirPath, "playlist.m3u8"), []byte(rewrittenManifest), 0644)
	} else {
		// Not master, proceed with normal rewriting (treat as video_0)
		// This path is likely unused if we always start with master, but good for safety
		rewrittenManifest, _ = p.cachePlaylist("playlist", p.Manifest, p.BaseURL, "video_0_")
	}

	p.Manifest = rewrittenManifest
	p.IsManifestRewritten = true
	return rewrittenManifest
}

// ServePlaylist serves the m3u8 manifest with rewritten URLs
func (p *RemoteHandler) ServePlaylist(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Master Playlist
	if path == "/playlist.m3u8" || path == "/media.mp4" {
		p.serveMasterPlaylist(w, r)
		return
	}

	// Video Track Playlist
	if strings.HasPrefix(path, "/video_") {
		// Extract index
		parts := strings.Split(strings.TrimSuffix(path, ".m3u8"), "_")
		if len(parts) == 2 {
			index, err := strconv.Atoi(parts[1])
			if err == nil {
				p.serveVideoPlaylist(w, r, index)
				return
			}
		}
	}

	// Audio Track Playlist
	if strings.HasPrefix(path, "/audio_") {
		// Extract index
		parts := strings.Split(strings.TrimSuffix(path, ".m3u8"), "_")
		if len(parts) == 2 {
			index, err := strconv.Atoi(parts[1])
			if err == nil {
				p.serveAudioPlaylist(w, r, index)
				return
			}
		}
	}

	http.NotFound(w, r)
}

// serveMasterPlaylist serves the master playlist with rewritten URLs
func (p *RemoteHandler) serveMasterPlaylist(w http.ResponseWriter, _ *http.Request) {
	// Briefly inhibit sleep on streaming requests (auto-stops after 30s of inactivity)
	inhibitor.Refresh(3 * time.Second)

	// Rewrite and Cache
	// For master playlist, we don't have a segment prefix per se, but we want to map tracks?
	// Actually, master playlist doesn't have segments, it has tracks.
	// rewriteMasterPlaylist handles track indices.
	// cachePlaylist uses rewriteManifestWithMap which is for segments.
	// We should probably just use rewriteMasterPlaylist here and cache it manually if needed.
	// But wait, the user asked to "write it to cache, write the raw one to cache AND write a json to cache that maps the indices to raw URLs".
	// For master playlist, the "indices" are track indices.
	// So we should probably implement a similar logic for master playlist.

	// Save Raw
	p.cacheMasterPlaylist("playlist", p.Manifest, p.BaseURL)

	rewrittenManifest := p.GetServedManifest()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")

	w.Write([]byte(rewrittenManifest))
}

// cacheMasterPlaylist caches the master playlist and its track map
func (p *RemoteHandler) cacheMasterPlaylist(name string, content string, baseURL string) {
	// Create directory
	dirPath := filepath.Join(p.CacheDir, name)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		logger.Logger.Error("Failed to create master playlist directory", "err", err)
		return
	}

	// 1. Save Raw
	rawPath := filepath.Join(dirPath, "raw.m3u8")
	if err := os.WriteFile(rawPath, []byte(content), 0644); err != nil {
		logger.Logger.Error("Failed to save raw master playlist", "err", err)
	}

	// 2. Generate Map (Track Indices -> URLs)
	// We can reuse ExtractTracksFromMaster logic
	mi := hls.ExtractTracksFromMaster(content)

	// We need to map video_0 -> URL, audio_0 -> URL
	// Let's create a map structure
	type MasterMap struct {
		Video []string `json:"video"`
		Audio []string `json:"audio"`
	}

	mm := MasterMap{
		Video: make([]string, len(mi.VideoTracks)),
		Audio: make([]string, len(mi.AudioTracks)),
	}

	for i, t := range mi.VideoTracks {
		mm.Video[i] = hls.ResolveURL(baseURL, t.URI)
	}
	for i, t := range mi.AudioTracks {
		mm.Audio[i] = hls.ResolveURL(baseURL, t.URI)
	}

	// 3. Save Map
	mapPath := filepath.Join(dirPath, "map.json")
	mapData, err := json.MarshalIndent(mm, "", "  ")
	if err == nil {
		if err := os.WriteFile(mapPath, mapData, 0644); err != nil {
			logger.Logger.Error("Failed to save master map", "err", err)
		}
	}

	// 4. Save Rewritten (we get it from GetServedManifest which calls rewriteMasterPlaylist)
	// We can't easily get it here without calling rewriteMasterPlaylist again or relying on p.Manifest if updated.
	// But GetServedManifest updates p.Manifest.
	// Let's just save what we serve.
	// The caller calls GetServedManifest.
}

// serveVideoPlaylist serves a specific video track
func (p *RemoteHandler) serveVideoPlaylist(w http.ResponseWriter, r *http.Request, index int) {
	// We need to get the URL for this track
	// If the original was a master playlist, we extract the track URL.
	// If the original was a media playlist, and index is 0, we use the original URL.

	targetURL := ""

	// Use the *original* manifest to extract tracks, as p.Manifest might be rewritten
	manifestToUse := p.OriginalManifest
	if manifestToUse == "" {
		manifestToUse = p.Manifest
	}

	playlistType := hls.ParsePlaylistType(manifestToUse)
	if playlistType == hls.PlaylistTypeMaster {
		mi := hls.ExtractTracksFromMaster(manifestToUse)
		if index >= 0 && index < len(mi.VideoTracks) {
			targetURL = hls.ResolveURL(p.BaseURL, mi.VideoTracks[index].URI)
		} else {
			http.Error(w, "Track not found", http.StatusNotFound)
			return
		}
	} else {
		if index == 0 {
			targetURL = p.BaseURL // It's the manifest itself
		} else {
			http.Error(w, "Track not found", http.StatusNotFound)
			return
		}
	}

	var playlistContent string

	// Let's assume we need to download it if it's a track from master.
	if targetURL != "" {
		// Download
		resp, err := p.downloadFile(r.Context(), targetURL)
		if err != nil {
			http.Error(w, "Failed to download playlist", http.StatusBadGateway)
			logger.Logger.Error("Failed to download playlist", "err", err, "url", targetURL)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		playlistContent = string(body)
	} else {
		// Use p.Manifest (only if media playlist)
		playlistContent = p.Manifest
		targetURL = p.BaseURL // Approximation
	}

	// Rewrite and Cache
	baseURL := targetURL
	if idx := strings.LastIndex(targetURL, "/"); idx != -1 {
		baseURL = targetURL[:idx+1]
	}

	rewritten, err := p.cachePlaylist(fmt.Sprintf("video_%d", index), playlistContent, baseURL, fmt.Sprintf("video_%d_", index))
	if err != nil {
		logger.Logger.Error("Failed to cache playlist", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rewritten = p.ensureHLSTags(rewritten)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(rewritten))
}

// serveAudioPlaylist serves a specific audio track
func (p *RemoteHandler) serveAudioPlaylist(w http.ResponseWriter, r *http.Request, index int) {
	manifestToUse := p.OriginalManifest
	if manifestToUse == "" {
		manifestToUse = p.Manifest
	}

	playlistType := hls.ParsePlaylistType(manifestToUse)
	if playlistType != hls.PlaylistTypeMaster {
		http.Error(w, "No audio tracks in media playlist", http.StatusNotFound)
		return
	}

	mi := hls.ExtractTracksFromMaster(manifestToUse)
	if index < 0 || index >= len(mi.AudioTracks) {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	targetURL := hls.ResolveURL(p.BaseURL, mi.AudioTracks[index].URI)

	// Download
	resp, err := p.downloadFile(r.Context(), targetURL)
	if err != nil {
		http.Error(w, "Failed to download playlist", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	playlistContent := string(body)

	// Rewrite and Cache
	baseURL := targetURL
	if idx := strings.LastIndex(targetURL, "/"); idx != -1 {
		baseURL = targetURL[:idx+1]
	}

	rewritten, err := p.cachePlaylist(fmt.Sprintf("audio_%d", index), playlistContent, baseURL, fmt.Sprintf("audio_%d_", index))
	if err != nil {
		logger.Logger.Error("Failed to cache playlist", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rewritten = p.ensureHLSTags(rewritten)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(rewritten))
}

// rewriteMasterPlaylist rewrites the master playlist to point to local endpoints
func (p *RemoteHandler) rewriteMasterPlaylist(manifest string) string {
	lines := strings.Split(manifest, "\n")
	var result []string

	// We need to track indices
	videoIdx := 0
	audioIdx := 0

	// We can't just increment on every line, we need to match how ExtractTracksFromMaster counts.
	// ExtractTracksFromMaster iterates and finds EXT-X-STREAM-INF for video, and EXT-X-MEDIA TYPE=AUDIO for audio.

	// Let's do a pass to rewrite.
	// Note: This simple parsing must match ExtractTracksFromMaster's logic to ensure indices align.

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#EXT-X-MEDIA") && strings.Contains(trimmed, "TYPE=AUDIO") {
			// It's an audio track
			// Replace URI="..." with /audio_{index}.m3u8
			if strings.Contains(trimmed, `URI="`) {
				uriPattern := regexp.MustCompile(`URI="([^"]+)"`)
				line = uriPattern.ReplaceAllString(trimmed, fmt.Sprintf(`URI="/audio_%d.m3u8"`, audioIdx))
				audioIdx++
			}
			result = append(result, line)
			continue
		}

		if strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF") {
			// The next line (or URI attribute) is the video playlist
			result = append(result, line)

			// Check if next line is URL
			if i+1 < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "#") {
				i++
				result = append(result, fmt.Sprintf("/video_%d.m3u8", videoIdx))
				videoIdx++
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// cachePlaylist saves raw/rewritten playlists and a segment map
func (p *RemoteHandler) cachePlaylist(name string, content string, baseURL string, segmentPrefix string) (string, error) {
	// Create directory
	dirPath := filepath.Join(p.CacheDir, name)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	// 1. Save Raw
	rawPath := filepath.Join(dirPath, "raw.m3u8")
	if err := os.WriteFile(rawPath, []byte(content), 0644); err != nil {
		logger.Logger.Error("Failed to save raw playlist", "err", err, "path", rawPath)
	}

	// 2. Rewrite and Generate Map
	rewritten, segmentMap, err := p.rewriteManifestWithMap(content, baseURL, segmentPrefix)
	if err != nil {
		return "", err
	}

	// 3. Save Map
	mapPath := filepath.Join(dirPath, "map.json")
	mapData, err := json.Marshal(segmentMap)
	if err == nil {
		if err := os.WriteFile(mapPath, mapData, 0644); err != nil {
			logger.Logger.Error("Failed to save segment map", "err", err, "path", mapPath)
		}
	} else {
		logger.Logger.Error("Failed to marshal segment map", "err", err)
	}

	// 4. Save Rewritten
	rewrittenPath := filepath.Join(dirPath, "playlist.m3u8")
	if err := os.WriteFile(rewrittenPath, []byte(rewritten), 0644); err != nil {
		logger.Logger.Error("Failed to save rewritten playlist", "err", err, "path", rewrittenPath)
	}

	return rewritten, nil
}

// rewriteManifestWithMap rewrites URLs and returns the rewritten manifest and a map of index -> absolute URL
func (p *RemoteHandler) rewriteManifestWithMap(manifest string, baseURL string, segmentPrefix string) (string, []string, error) {
	lines := strings.Split(manifest, "\n")
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", nil, err
	}

	var segmentMap []string
	var rewrittenLines []string

	// segmentPrefix is expected to be like "video_0_" or "audio_0_"
	pathPrefix := ""
	if strings.HasPrefix(segmentPrefix, "video_") {
		parts := strings.Split(segmentPrefix, "_")
		if len(parts) >= 2 {
			pathPrefix = fmt.Sprintf("/video_%s/segment_", parts[1])
		}
	} else if strings.HasPrefix(segmentPrefix, "audio_") {
		parts := strings.Split(segmentPrefix, "_")
		if len(parts) >= 2 {
			pathPrefix = fmt.Sprintf("/audio_%s/segment_", parts[1])
		}
	}

	if pathPrefix == "" {
		pathPrefix = "/segment/" + segmentPrefix
	}

	segmentCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip if already rewritten
		if strings.HasPrefix(trimmed, "/video_") || strings.HasPrefix(trimmed, "/audio_") {
			rewrittenLines = append(rewrittenLines, line)
			continue
		}

		// Check for segment URLs (lines not starting with #)
		if !strings.HasPrefix(trimmed, "#") {
			// Resolve to absolute URL
			u, err := url.Parse(trimmed)
			absoluteURL := trimmed
			if err == nil {
				absoluteURL = base.ResolveReference(u).String()
			}

			segmentMap = append(segmentMap, absoluteURL)
			rewrittenLines = append(rewrittenLines, fmt.Sprintf("%s%d.ts", pathPrefix, segmentCount))
			segmentCount++
		} else {
			// Handle URI="..." for encryption keys etc
			if strings.Contains(line, `URI="`) && !strings.Contains(line, "TYPE=AUDIO") {
				uriPattern := regexp.MustCompile(`URI="([^"]+)"`)
				line = uriPattern.ReplaceAllStringFunc(line, func(match string) string {
					path := uriPattern.FindStringSubmatch(match)[1]
					if strings.HasPrefix(path, "http") {
						return match
					}
					u, err := url.Parse(path)
					if err == nil {
						return fmt.Sprintf(`URI="%s"`, base.ResolveReference(u).String())
					}
					return match
				})
			}
			rewrittenLines = append(rewrittenLines, line)
		}
	}

	return strings.Join(rewrittenLines, "\n"), segmentMap, nil
}

// ensureHLSTags adds required HLS tags if missing for better Chromecast compatibility
func (p *RemoteHandler) ensureHLSTags(playlist string) string {
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
