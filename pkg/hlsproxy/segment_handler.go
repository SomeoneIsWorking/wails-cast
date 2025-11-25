package hlsproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleSegment proxies segment requests with captured cookies and headers,
// and transcodes them using ffmpeg for compatibility
func (p *HLSProxy) handleSegment(w http.ResponseWriter, r *http.Request) {
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

		p.serveSegment(w, fullURL, key)
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
				p.serveSegment(w, fullURL, key)
				return
			}
			segmentCount++
		}
	}

	http.Error(w, "Segment index out of range", http.StatusNotFound)
}

// serveSegment serves a segment by URL and key
func (p *HLSProxy) serveSegment(w http.ResponseWriter, fullURL string, key string) {
	fmt.Printf("Proxying request: %s (Key: %s)\n", fullURL, key)

	// Check manifest
	p.ManifestData.mu.RLock()
	item, exists := p.ManifestData.Items[fullURL]
	p.ManifestData.mu.RUnlock()

	if exists && item.LocalPath != "" {
		fmt.Printf("Found in manifest: %s (Playlist: %v, Transcoded: %v, Type: %s)\n", fullURL, item.IsPlaylist, item.Transcoded, item.ContentType)
		if item.IsPlaylist {
			p.servePlaylist(w, item.LocalPath, fullURL)
			return
		}
		if item.Transcoded {
			p.serveFile(w, item.TranscodedPath, "video/mp2t")
			return
		}
		// For non-playlist items, always try to transcode
		// This handles disguised video segments (e.g., .jpg files that are actually video)
		p.transcodeAndServe(w, item)
		return
	}

	// Not in manifest, download it
	fmt.Println("Not in manifest, downloading...")

	// Create a cache key
	cacheKey := key

	localPath := filepath.Join(p.CacheDir, cacheKey+".ts")

	resp, err := p.downloadFile(fullURL)
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
	p.transcodeAndServe(w, newItem)
}

// serveFile serves a local file
func (p *HLSProxy) serveFile(w http.ResponseWriter, path string, contentType string) {
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
func (p *HLSProxy) transcodeAndServe(w http.ResponseWriter, item *ManifestItem) {
	transcodedPath := item.LocalPath + "_transcoded.ts"

	// If already transcoded (but maybe flag wasn't set or we are retrying), check file
	if _, err := os.Stat(transcodedPath); err == nil {
		item.TranscodedPath = transcodedPath
		item.Transcoded = true
		p.updateManifest(item.URL, item)
		p.serveFile(w, transcodedPath, "video/mp2t")
		return
	}

	fmt.Println("Transcoding segment...")

	// First try: copy video codec if it's already H.264, only transcode audio
	// This is much faster and avoids quality loss
	cmd := exec.Command("ffmpeg",
		"-i", item.LocalPath,
		"-c:v", "copy", // Try copying video
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "48000",
		"-ac", "2",
		"-f", "mpegts",
		"-mpegts_copyts", "1",
		"-y",
		transcodedPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		// Success with video copy, audio transcode
		item.TranscodedPath = transcodedPath
		item.Transcoded = true
		p.updateManifest(item.URL, item)
		fmt.Println("Transcoding (video copy, audio re-encode) complete, serving...")
		p.serveFile(w, transcodedPath, "video/mp2t")
		return
	}

	fmt.Printf("First transcode attempt (video copy) failed: %v\nStderr: %s\n", err, stderr.String())
	fmt.Println("Falling back to full video and audio re-encode...")

	// Use Chromecast-compatible settings:
	// - H.264 High Profile Level 4.2 (max supported by most Chromecasts)
	// - AAC-LC audio at 128kbps, 48kHz (recommended for Chromecast)
	// - Proper MPEG-TS muxing with alignment
	cmd = exec.Command("ffmpeg",
		"-i", item.LocalPath,
		"-c:v", "libx264",
		"-profile:v", "high",
		"-level", "4.2",
		"-preset", "veryfast", // Fast encoding
		"-pix_fmt", "yuv420p", // Chromecast requires 4:2:0
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "48000", // 48kHz sample rate
		"-ac", "2", // Stereo
		"-f", "mpegts",
		"-mpegts_copyts", "1",
		"-y",
		transcodedPath,
	)

	stderr.Reset()
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Transcode failed: %v\nStderr: %s\n", err, stderr.String())
		// Fallback to original if transcode fails? Or error?
		// User said "it should be transcoded then served".
		// If fail, maybe serve original as fallback to keep stream alive?
		// Let's try serving original if transcode fails.
		// Force video/mp2t as we know it's intended to be video
		p.serveFile(w, item.LocalPath, "video/mp2t")
		return
	}

	item.TranscodedPath = transcodedPath
	item.Transcoded = true
	p.updateManifest(item.URL, item)

	fmt.Println("Transcoding complete, serving...")
	p.serveFile(w, transcodedPath, "video/mp2t")
}
