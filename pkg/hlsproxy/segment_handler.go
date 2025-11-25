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
				p.transcodeAndServe(w, item)
				return
			}
		}
	}

	// Not in manifest, download it
	fmt.Printf("⚠️ Not in manifest, downloading: %s (Key: %s)\n", fullURL, key)

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
	// transcodedPath already has .ts extension from line 188

	// Determine if it's an audio or video segment based on filename
	isAudio := strings.Contains(filepath.Base(item.LocalPath), "audio_")
	isVideo := strings.Contains(filepath.Base(item.LocalPath), "video_")

	args := []string{
		"-copyts", // IMPORTANT: Preserve input timestamps!
		"-i", item.LocalPath,
		"-y",
	}

	if isAudio {
		// Audio segment: Map only audio, re-encode to ensure compatibility, drop ID3/video
		args = append(args,
			"-map", "0:a",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-ac", "2",
		)
	} else if isVideo {
		// Video segment: Map only video, copy stream, drop ID3/audio
		args = append(args,
			"-map", "0:v",
			"-c:v", "copy",
		)
	} else {
		// Fallback for unknown types (e.g. muxed): Try to handle both
		args = append(args,
			"-c:v", "copy",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-ac", "2",
		)
	}

	// Common flags
	args = append(args,
		"-f", "mpegts",
		"-mpegts_copyts", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		transcodedPath,
	)

	cmd := exec.Command("ffmpeg", args...)

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
	// Use Chromecast-compatible settings:
	// - H.264 High Profile Level 4.2 (max supported by most Chromecasts)
	// - AAC-LC audio at 128kbps, 48kHz (recommended for Chromecast)
	// - Proper MPEG-TS muxing with alignment

	args = []string{
		"-copyts", // IMPORTANT: Preserve input timestamps!
		"-i", item.LocalPath,
		"-y",
	}

	if isAudio {
		args = append(args,
			"-map", "0:a",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-ac", "2",
		)
	} else if isVideo {
		// If video copy failed, re-encode video
		args = append(args,
			"-map", "0:v",
			"-c:v", "libx264",
			"-profile:v", "high",
			"-level", "4.2",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
		)
	} else {
		args = append(args,
			"-c:v", "libx264",
			"-profile:v", "high",
			"-level", "4.2",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-ac", "2",
		)
	}

	args = append(args,
		"-f", "mpegts",
		"-mpegts_copyts", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		transcodedPath,
	)

	fmt.Printf("🎥 FFMPEG Command: ffmpeg %s\n", strings.Join(args, " "))
	cmd = exec.Command("ffmpeg", args...)

	stderr.Reset()
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Transcode failed: %v\nStderr: %s\n", err, stderr.String())
		// Fallback to original if transcode fails
		p.serveFile(w, item.LocalPath, "video/mp2t")
		return
	}

	item.TranscodedPath = transcodedPath
	item.Transcoded = true
	p.updateManifest(item.URL, item)

	fmt.Println("Transcoding complete, serving...")
	p.serveFile(w, transcodedPath, "video/mp2t")
}
