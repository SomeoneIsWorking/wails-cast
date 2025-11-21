package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HLSSession represents an active HLS streaming session
type HLSSession struct {
	VideoPath    string
	SubtitlePath string
	OutputDir    string
	Duration     float64 // Total video duration in seconds
	SegmentSize  int     // Segment duration in seconds
	LocalIP      string
}

// NewHLSSession creates a new HLS session
func NewHLSSession(videoPath, subtitlePath, localIP string) *HLSSession {
	// Get video duration
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		logger.Warn("Failed to get video duration", "error", err)
		duration = 0
	}

	// Create output directory
	sessionID := filepath.Base(videoPath)
	baseDir := filepath.Join(os.TempDir(), "wails-cast-hls")
	outputDir := filepath.Join(baseDir, sessionID)
	os.MkdirAll(outputDir, 0755)

	return &HLSSession{
		VideoPath:    videoPath,
		SubtitlePath: subtitlePath,
		OutputDir:    outputDir,
		Duration:     duration,
		SegmentSize:  15,
		LocalIP:      localIP,
	}
}

// ServePlaylist generates and serves the HLS playlist dynamically
func (s *HLSSession) ServePlaylist(w http.ResponseWriter, r *http.Request) {
	// Generate complete playlist with all segments
	var playlist strings.Builder

	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:3\n")
	playlist.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", s.SegmentSize))
	playlist.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	playlist.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	// Calculate number of segments
	numSegments := int(s.Duration / float64(s.SegmentSize))
	if float64(numSegments*s.SegmentSize) < s.Duration {
		numSegments++
	}

	logger.Info("Generating playlist", "duration", s.Duration, "segmentSize", s.SegmentSize, "numSegments", numSegments)

	// Add all segments with proper durations
	for i := 0; i < numSegments; i++ {
		segmentDuration := float64(s.SegmentSize)
		// Last segment might be shorter
		if i == numSegments-1 {
			remaining := s.Duration - float64(i*s.SegmentSize)
			if remaining < float64(s.SegmentSize) {
				segmentDuration = remaining
			}
		}

		playlist.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segmentDuration))
		playlist.WriteString(fmt.Sprintf("http://%s:8888/segment%d.ts\n", s.LocalIP, i))
	}

	playlist.WriteString("#EXT-X-ENDLIST\n")

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")

	w.Write([]byte(playlist.String()))
}

// ServeSegment transcodes and serves a specific segment on-demand
func (s *HLSSession) ServeSegment(w http.ResponseWriter, r *http.Request, segmentName string) {
	logger.Info("ServeSegment called", "segmentName", segmentName, "videoPath", s.VideoPath)
	// Extract segment number from name (e.g., "segment123.ts" -> 123)
	segmentNum, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(segmentName, "segment"), ".ts"))
	if err != nil {
		logger.Error("Invalid segment name format", "segmentName", segmentName, "error", err)
		http.Error(w, "Invalid segment name", http.StatusBadRequest)
		return
	}

	logger.Info("Segment number parsed", "segmentNum", segmentNum)
	segmentPath := filepath.Join(s.OutputDir, segmentName)
	logger.Info("Checking segment path", "segmentPath", segmentPath)

	// Calculate segment duration
	segmentDuration := float64(s.SegmentSize)
	startTime := float64(segmentNum * s.SegmentSize)
	if startTime+segmentDuration > s.Duration {
		segmentDuration = s.Duration - startTime
	}

	// Check manifest to see if regeneration is needed
	needsRegeneration := false
	manifest, err := loadSegmentManifest(s.OutputDir, segmentNum)
	if err != nil || !manifestMatches(manifest, s.SubtitlePath, segmentDuration) {
		logger.Info("Segment needs regeneration", "segment", segmentNum, "reason", "manifest mismatch or missing")
		needsRegeneration = true
		// Remove old segment if it exists
		os.Remove(segmentPath)
	}

	// Check if segment file already exists
	if _, err := os.Stat(segmentPath); os.IsNotExist(err) || needsRegeneration {
		logger.Info("Segment file does not exist or needs regeneration, will transcode", "segmentPath", segmentPath)
		// Wait briefly to see if connection stays alive (avoid transcoding if seeking rapidly)
		select {
		case <-r.Context().Done():
			// Client disconnected/cancelled - don't transcode
			logger.Info("Segment request cancelled, skipping transcode", "segment", segmentNum)
			return
		case <-time.After(100 * time.Millisecond):
			// Connection still alive, proceed with transcode
		}

		// Transcode this segment on-demand
		logger.Info("Transcoding segment on-demand", "segment", segmentNum)

		// Build FFmpeg command to extract just this segment
		args := []string{
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%d", s.SegmentSize),
			"-i", s.VideoPath,
			"-copyts",
		}

		// Video encoding
		args = append(args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-tune", "zerolatency",
			"-pix_fmt", "yuv420p",
		)

		// Add subtitles if provided
		if s.SubtitlePath != "" {
			// Check if it's an embedded subtitle track (format: "videopath:si=N")
			if strings.Contains(s.SubtitlePath, ":si=") {
				// Embedded subtitle track - extract stream index
				parts := strings.Split(s.SubtitlePath, ":si=")
				if len(parts) == 2 {
					streamIndex := parts[1]
					// Properly construct the subtitles filter with stream index
					filterStr := fmt.Sprintf("subtitles=%s:si=%s:force_style='FontSize=24'", s.VideoPath, streamIndex)
					args = append(args, "-vf", filterStr)
					logger.Info("Using embedded subtitle track", "streamIndex", streamIndex, "filter", filterStr)
				}
			} else if _, err := os.Stat(s.SubtitlePath); err == nil {
				// External subtitle file
				escapedPath := strings.ReplaceAll(s.SubtitlePath, "\\", "/")
				escapedPath = strings.ReplaceAll(escapedPath, ":", "\\\\:")
				args = append(args, "-vf", fmt.Sprintf("subtitles=%s:force_style='FontSize=24'", escapedPath))
			}
		}

		// Audio encoding
		args = append(args,
			"-c:a", "aac",
			"-b:a", "192k",
			"-ac", "2",
		)

		// Output as MPEG-TS
		args = append(args,
			"-f", "mpegts",
			segmentPath,
		)

		logger.Info("FFMPEG CALL: ffmpeg " + strings.Join(args, " "))
		cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
		if err := cmd.Run(); err != nil {
			// Check if it was cancelled
			if r.Context().Err() != nil {
				logger.Info("Transcode cancelled", "segment", segmentNum)
				return
			}
			logger.Error("Failed to transcode segment", "error", err, "segment", segmentNum)
			http.Error(w, "Transcode failed", http.StatusInternalServerError)
			return
		}

		logger.Info("Segment transcoded", "segment", segmentNum)

		// Save manifest for this segment
		manifest := SegmentManifest{
			SegmentNumber: segmentNum,
			Duration:      segmentDuration,
			SubtitlePath:  s.SubtitlePath,
			SubtitleStyle: "FontSize=24",
			VideoCodec:    "libx264",
			AudioCodec:    "aac",
			Preset:        "veryfast",
			CreatedAt:     time.Now().Format(time.RFC3339),
		}
		if err := saveSegmentManifest(s.OutputDir, manifest); err != nil {
			logger.Warn("Failed to save segment manifest", "error", err, "segment", segmentNum)
		}
	} else {
		logger.Info("Segment file exists, serving cached version", "segmentPath", segmentPath)
	}

	// Serve the segment
	logger.Info("Serving segment file", "segmentPath", segmentPath)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache segments

	http.ServeFile(w, r, segmentPath)
}

// Cleanup removes session files
func (s *HLSSession) Cleanup() {
	if s.OutputDir != "" {
		os.RemoveAll(s.OutputDir)
	}
}
