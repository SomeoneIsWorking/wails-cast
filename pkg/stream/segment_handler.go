package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-cast/pkg/hls"
)

// ServeSegment proxies segment requests with captured cookies and headers,
// and transcodes them using ffmpeg for compatibility
func (p *RemoteHandler) ServeSegment(w http.ResponseWriter, r *http.Request) {
	// Briefly inhibit sleep on streaming requests (auto-stops after 30s of inactivity)
	inhibitor.Refresh(3 * time.Second)

	// Optional: Wait briefly to see if connection stays alive (avoid transcoding if seeking rapidly)
	select {
	case <-r.Context().Done():
		// Client disconnected/cancelled - don't transcode
		return
	case <-time.After(100 * time.Millisecond):
		// Connection still alive, proceed with transcode
	}

	path := r.URL.Path
	// Expected formats:
	// /video_{trackIdx}/segment_{segIdx}.ts
	// /video_{trackIdx}/segment_{segIdx}_raw.ts
	// /audio_{trackIdx}/segment_{segIdx}.ts
	// /audio_{trackIdx}/segment_{segIdx}_raw.ts

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path structure", http.StatusBadRequest)
		return
	}

	trackPart := parts[0]   // e.g. video_0
	segmentPart := parts[1] // e.g. segment_1.ts or segment_1_raw.ts

	// Parse track type and index
	var trackType string
	var trackIdx int
	if strings.HasPrefix(trackPart, "video_") {
		trackType = "video"
		fmt.Sscanf(strings.TrimPrefix(trackPart, "video_"), "%d", &trackIdx)
	} else if strings.HasPrefix(trackPart, "audio_") {
		trackType = "audio"
		fmt.Sscanf(strings.TrimPrefix(trackPart, "audio_"), "%d", &trackIdx)
	} else {
		http.Error(w, "Invalid track type", http.StatusBadRequest)
		return
	}

	// Parse segment index and raw flag
	isRaw := strings.Contains(segmentPart, "_raw.ts")
	var segIdx int
	// Remove extension and prefix
	segStr := strings.TrimPrefix(segmentPart, "segment_")
	if isRaw {
		segStr = strings.TrimSuffix(segStr, "_raw.ts")
	} else {
		segStr = strings.TrimSuffix(segStr, ".ts")
	}

	if _, err := fmt.Sscanf(segStr, "%d", &segIdx); err != nil {
		http.Error(w, "Invalid segment index", http.StatusBadRequest)
		return
	}

	// Resolve segment URL from playlist
	fullURL, err := p.resolveSegmentURL(trackType, trackIdx, segIdx)
	if err != nil {
		fmt.Printf("❌ Failed to resolve segment URL: %v\n", err)
		http.Error(w, "Segment not found", http.StatusNotFound)
		return
	}

	// Create a unique key for this segment
	key := fmt.Sprintf("%s_%d_segment_%d", trackType, trackIdx, segIdx)

	p.serveSegment(w, r, fullURL, key, isRaw, trackType, trackIdx, segIdx)
}

// resolveSegmentURL finds the absolute URL of a segment by index
func (p *RemoteHandler) resolveSegmentURL(trackType string, trackIdx int, segIdx int) (string, error) {
	// 1. Try to load from JSON map (fastest, most reliable)
	// Map is now at cache/track_idx/map.json
	trackDir := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, trackIdx))
	mapPath := filepath.Join(trackDir, "map.json")

	if mapData, err := os.ReadFile(mapPath); err == nil {
		var segmentMap []string
		if err := json.Unmarshal(mapData, &segmentMap); err == nil {
			if segIdx >= 0 && segIdx < len(segmentMap) {
				return segmentMap[segIdx], nil
			}
			return "", fmt.Errorf("segment index %d out of range (map size: %d)", segIdx, len(segmentMap))
		}
		fmt.Printf("⚠️ Failed to parse segment map %s: %v\n", mapPath, err)
	}

	// 2. Fallback to parsing the cached playlist (slower, fragile)
	// Playlist is now at cache/track_idx/playlist.m3u8
	playlistPath := filepath.Join(trackDir, "playlist.m3u8")

	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		return "", fmt.Errorf("playlist not cached: %s", playlistPath)
	}

	content, err := os.ReadFile(playlistPath)
	if err != nil {
		return "", err
	}

	// Parse playlist to find the Nth segment
	lines := strings.Split(string(content), "\n")
	segmentCount := 0

	// We need to resolve relative URLs against the playlist's base URL.
	// But wait, we don't have the base URL handy here easily without re-extracting.
	// However, if we are in fallback mode, it means we failed to generate the map?
	// Or maybe the map generation failed?
	// Let's try to re-derive the track URL.

	trackURL := ""
	// Use OriginalManifest if available
	manifestToUse := p.OriginalManifest
	if manifestToUse == "" {
		manifestToUse = p.Manifest
	}

	if hls.ParsePlaylistType(manifestToUse) == hls.PlaylistTypeMaster {
		mi := hls.ExtractTracksFromMaster(manifestToUse)
		if trackType == "video" && trackIdx < len(mi.VideoTracks) {
			trackURL = hls.ResolveURL(p.BaseURL, mi.VideoTracks[trackIdx].URI)
		} else if trackType == "audio" && trackIdx < len(mi.AudioTracks) {
			trackURL = hls.ResolveURL(p.BaseURL, mi.AudioTracks[trackIdx].URI)
		}
	} else {
		// Single stream
		trackURL = p.BaseURL
	}

	if trackURL == "" {
		return "", fmt.Errorf("could not determine track URL")
	}

	trackBaseURL := trackURL
	if idx := strings.LastIndex(trackURL, "/"); idx != -1 {
		trackBaseURL = trackURL[:idx+1]
	}

	base, err := url.Parse(trackBaseURL)
	if err != nil {
		return "", err
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") && trimmed != "" {
			if segmentCount == segIdx {
				// Resolve URL
				u, err := url.Parse(trimmed)
				if err == nil {
					return base.ResolveReference(u).String(), nil
				}
				return trimmed, nil // Fallback
			}
			segmentCount++
		}
	}

	return "", fmt.Errorf("segment index %d out of range", segIdx)
}

// serveSegment serves a segment by URL and key
func (p *RemoteHandler) serveSegment(w http.ResponseWriter, r *http.Request, fullURL string, key string, isRaw bool, trackType string, trackIdx int, segIdx int) {
	fmt.Printf("Proxying request: %s (Key: %s, Raw: %v)\n", fullURL, key, isRaw)

	// Check manifest
	p.ManifestData.mu.RLock()
	item, exists := p.ManifestData.Items[fullURL]
	p.ManifestData.mu.RUnlock()

	if exists && item.LocalPath != "" {
		// Verify file exists on disk
		if _, err := os.Stat(item.LocalPath); err != nil {
			fmt.Printf("⚠️ File missing on disk: %s. Re-downloading.\n", item.LocalPath)
			// Fall through to download logic
		} else {
			// File exists
			if isRaw {
				// Serve raw file
				p.serveFile(w, item.LocalPath, "video/mp2t")
				return
			}

			// Serve transcoded
			if item.Transcoded {
				if _, err := os.Stat(item.TranscodedPath); err == nil {
					p.serveFile(w, item.TranscodedPath, "video/mp2t")
					return
				}
			}

			// Need to transcode
			p.transcodeAndServe(w, r, item)
			return
		}
	}

	// Not in manifest or missing, download it
	fmt.Printf("⚠️ Not in manifest/disk, downloading: %s\n", fullURL)

	// Create a cache key
	// Store in directory: cache/track_idx/segment_idx.ts
	trackDir := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, trackIdx))
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create track directory: %v", err), http.StatusInternalServerError)
		return
	}

	localPath := filepath.Join(trackDir, fmt.Sprintf("segment_%d.ts", segIdx))

	resp, err := p.downloadFile(r.Context(), fullURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to download: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

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

	p.updateManifest(fullURL, newItem)

	if isRaw {
		p.serveFile(w, localPath, "video/mp2t")
	} else {
		p.transcodeAndServe(w, r, newItem)
	}
}

// serveFile serves a local file
func (p *RemoteHandler) serveFile(w http.ResponseWriter, path string, contentType string) {
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
func (p *RemoteHandler) transcodeAndServe(w http.ResponseWriter, r *http.Request, item *ManifestItem) {
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

	// Build transcode options using shared module
	opts := hls.TranscodeOptions{
		InputPath:   item.LocalPath,
		OutputPath:  transcodedPath,
		IsAudioOnly: isAudio,
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
