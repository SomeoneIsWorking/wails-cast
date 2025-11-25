package stream

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-cast/pkg/hls"
)

// handleSegment proxies segment requests with captured cookies and headers,
// and transcodes them using ffmpeg for compatibility
func (p *RemoteHLSProxy) handleSegment(w http.ResponseWriter, r *http.Request) {
	// Optional: Wait briefly to see if connection stays alive (avoid transcoding if seeking rapidly)
	select {
	case <-r.Context().Done():
		// Client disconnected/cancelled - don't transcode
		return
	case <-time.After(100 * time.Millisecond):
		// Connection still alive, proceed with transcode
	}
	// Parse key from path
	path := r.URL.Path
	if !strings.HasPrefix(path, "/segment/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	key := path[len("/segment/"):]
	// Remove extension
	if idx := strings.LastIndex(key, "."); idx != -1 {
		key = key[:idx]
	}

	// Parse key: e.g. "video_0" -> type="video", index=0
	var segType string
	var index int
	if strings.Contains(key, "_") {
		parts := strings.SplitN(key, "_", 2)
		segType = parts[0]
		fmt.Sscanf(parts[1], "%d", &index)
	} else {
		fmt.Sscanf(key, "%d", &index)
	}

	// Get URL from cached playlist
	var playlistPath string
	if segType == "audio" {
		playlistPath = filepath.Join(p.CacheDir, "audio.m3u8")
	} else if segType == "video" {
		playlistPath = filepath.Join(p.CacheDir, "video.m3u8")
	} else {
		// For regular segments, use the map
		p.ManifestData.mu.RLock()
		fullURL, ok := p.ManifestData.SegmentMap[key]
		p.ManifestData.mu.RUnlock()

		if !ok {
			fmt.Printf("❌ Segment not found for key: %s\n", key)
			http.Error(w, "Segment not found", http.StatusNotFound)
			return
		}

		p.serveSegment(w, r, fullURL, key)
		return
	}

	// Read playlist
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	// Parse playlist to find the index-th segment
	lines := strings.Split(string(content), "\n")
	segmentCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") && trimmed != "" {
			if segmentCount == index {
				fullURL := trimmed
				p.serveSegment(w, r, fullURL, key)
				return
			}
			segmentCount++
		}
	}

	http.Error(w, "Segment index out of range", http.StatusNotFound)
}

// serveSegment serves a segment by URL and key
func (p *RemoteHLSProxy) serveSegment(w http.ResponseWriter, r *http.Request, fullURL string, key string) {
	fmt.Printf("Proxying request: %s (Key: %s)\n", fullURL, key)

	// Check manifest
	p.ManifestData.mu.RLock()
	item, exists := p.ManifestData.Items[fullURL]
	p.ManifestData.mu.RUnlock()

	if exists && item.LocalPath != "" {
		// Verify file exists on disk (don't rely just on memory/manifest)
		if _, err := os.Stat(item.LocalPath); err != nil {
			fmt.Printf("⚠️ File missing on disk despite manifest entry: %s. Re-downloading.\n", item.LocalPath)
			// Fall through to download logic
		} else {
			fmt.Printf("Found in manifest: %s (Playlist: %v, Transcoded: %v, Type: %s)\n", fullURL, item.IsPlaylist, item.Transcoded, item.ContentType)
			if item.IsPlaylist {
				p.servePlaylist(w, item.LocalPath, fullURL)
				return
			}
			if item.Transcoded {
				// Verify transcoded file exists
				if _, err := os.Stat(item.TranscodedPath); err == nil {
					p.serveFile(w, item.TranscodedPath, "video/mp2t")
					return
				}
				fmt.Printf("⚠️ Transcoded file missing on disk: %s. Re-transcoding.\n", item.TranscodedPath)
				// Fall through to transcode logic (item.Transcoded will be overwritten)
			} else {
				// Not transcoded yet, or re-transcoding needed
				// For non-playlist items, always try to transcode
				p.transcodeAndServe(w, r, item)
				return
			}
		}
	}

	// Not in manifest, download it
	fmt.Printf("⚠️ Not in manifest, downloading: %s (Key: %s)\n", fullURL, key)

	// Create a cache key
	cacheKey := key

	localPath := filepath.Join(p.CacheDir, cacheKey+".ts")

	resp, err := p.downloadFile(r.Context(), fullURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to download: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	fmt.Printf("Content-Type: %s\n", contentType)

	// Save to file
	outFile, err := os.Create(localPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create file: %v", err), http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(outFile, resp.Body)
	outFile.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}

	// Create manifest item
	newItem := &ManifestItem{
		URL:         fullURL,
		ContentType: contentType,
		LocalPath:   localPath,
		Transcoded:  false,
	}

	// Check if playlist
	if strings.Contains(contentType, "application/vnd.apple.mpegurl") ||
		strings.Contains(contentType, "application/x-mpegURL") ||
		strings.HasSuffix(fullURL, ".m3u8") {
		newItem.IsPlaylist = true
		p.updateManifest(fullURL, newItem)
		p.servePlaylist(w, localPath, fullURL)
		return
	}

	// All other items are treated as segments and transcoded
	newItem.IsPlaylist = false
	p.updateManifest(fullURL, newItem)
	p.transcodeAndServe(w, r, newItem)
}

// serveFile serves a local file
func (p *RemoteHLSProxy) serveFile(w http.ResponseWriter, path string, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// transcodeAndServe transcodes a segment and serves it
func (p *RemoteHLSProxy) transcodeAndServe(w http.ResponseWriter, r *http.Request, item *ManifestItem) {
	transcodedPath := item.LocalPath + "_transcoded.ts"

	// If already transcoded, serve it
	if _, err := os.Stat(transcodedPath); err == nil {
		item.TranscodedPath = transcodedPath
		item.Transcoded = true
		p.updateManifest(item.URL, item)
		p.serveFile(w, transcodedPath, "video/mp2t")
		return
	}

	// Determine segment type based on filename
	isAudio := strings.Contains(filepath.Base(item.LocalPath), "audio_")
	isVideo := strings.Contains(filepath.Base(item.LocalPath), "video_")

	// Build transcode options using shared module
	opts := hls.TranscodeOptions{
		InputPath:   item.LocalPath,
		OutputPath:  transcodedPath,
		IsAudioOnly: isAudio,
		IsVideoOnly: isVideo,
		Preset:      "veryfast",
	}

	err := hls.TranscodeSegment(r.Context(), opts)

	if err != nil {
		return
	}

	item.TranscodedPath = transcodedPath
	item.Transcoded = true
	p.updateManifest(item.URL, item)
	fmt.Println("Transcoding complete, serving...")
	p.serveFile(w, transcodedPath, "video/mp2t")
}
