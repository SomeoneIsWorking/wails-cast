package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HLSManagerManual manages HLS streaming sessions (manual mode - on-demand segment generation)
type HLSManagerManual struct {
	sessions map[string]*HLSSessionManual
	mu       sync.RWMutex
	baseDir  string
	localIP  string
}

// HLSSessionManual represents an active HLS streaming session (manual mode)
type HLSSessionManual struct {
	ID           string
	VideoPath    string
	SubtitlePath string
	OutputDir    string
	Duration     float64 // Total video duration in seconds
	SegmentSize  int     // Segment duration in seconds
	mu           sync.RWMutex
	segments     map[int]bool // Track which segments have been transcoded
}

// NewHLSManagerManual creates a new HLS manager (manual mode)
func NewHLSManagerManual(localIP string) *HLSManagerManual {
	baseDir := filepath.Join(os.TempDir(), "wails-cast-hls")
	os.MkdirAll(baseDir, 0755)

	return &HLSManagerManual{
		sessions: make(map[string]*HLSSessionManual),
		baseDir:  baseDir,
		localIP:  localIP,
	}
}

// GetOrCreateSession gets existing session or creates new one
func (m *HLSManagerManual) GetOrCreateSession(videoPath, subtitlePath string, seekTime int) *HLSSessionManual {
	sessionID := fmt.Sprintf("%s_%d", filepath.Base(videoPath), seekTime)

	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session
	}

	// Get video duration
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		logger.Warn("Failed to get video duration", "error", err)
		duration = 0
	}

	// Create new session
	outputDir := filepath.Join(m.baseDir, sessionID)
	os.MkdirAll(outputDir, 0755)

	session := &HLSSessionManual{
		ID:           sessionID,
		VideoPath:    videoPath,
		SubtitlePath: subtitlePath,
		OutputDir:    outputDir,
		Duration:     duration,
		SegmentSize:  4, // 4-second segments
		segments:     make(map[int]bool),
	}

	m.sessions[sessionID] = session
	return session
}

// ServePlaylist generates and serves the HLS playlist dynamically
func (m *HLSManagerManual) ServePlaylist(w http.ResponseWriter, r *http.Request, session *HLSSessionManual) {
	// Generate complete playlist with all segments
	var playlist strings.Builder

	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:3\n")
	playlist.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", session.SegmentSize+1))
	playlist.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	playlist.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	// Calculate number of segments
	numSegments := int(session.Duration / float64(session.SegmentSize))
	if float64(numSegments*session.SegmentSize) < session.Duration {
		numSegments++
	}

	logger.Info("Generating playlist", "duration", session.Duration, "segmentSize", session.SegmentSize, "numSegments", numSegments)

	// Add all segments with proper durations
	for i := 0; i < numSegments; i++ {
		segmentDuration := float64(session.SegmentSize)
		// Last segment might be shorter
		if i == numSegments-1 {
			remaining := session.Duration - float64(i*session.SegmentSize)
			if remaining < float64(session.SegmentSize) {
				segmentDuration = remaining
			}
		}

		playlist.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segmentDuration))
		playlist.WriteString(fmt.Sprintf("http://%s:8888/segment%d.ts\n", m.localIP, i))
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
func (m *HLSManagerManual) ServeSegment(w http.ResponseWriter, r *http.Request, session *HLSSessionManual, segmentName string) {
	// Extract segment number from name (e.g., "segment123.ts" -> 123)
	segmentNum, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(segmentName, "segment"), ".ts"))
	if err != nil {
		http.Error(w, "Invalid segment name", http.StatusBadRequest)
		return
	}

	// Check if already transcoded
	session.mu.RLock()
	exists := session.segments[segmentNum]
	session.mu.RUnlock()

	segmentPath := filepath.Join(session.OutputDir, segmentName)

	if !exists {
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
		logger.Info("Transcoding segment on-demand", "session", session.ID, "segment", segmentNum)

		// Calculate start time for this segment
		startTime := float64(segmentNum * session.SegmentSize)

		// Build FFmpeg command to extract just this segment
		args := []string{
			"-ss", fmt.Sprintf("%.2f", startTime),
			"-t", fmt.Sprintf("%d", session.SegmentSize),
			"-i", session.VideoPath,
		}

		// Video encoding
		args = append(args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-pix_fmt", "yuv420p",
			"-profile:v", "main",
			"-g", "48",
		)

		// Add subtitles if provided
		if session.SubtitlePath != "" {
			if _, err := os.Stat(session.SubtitlePath); err == nil {
				escapedPath := strings.ReplaceAll(session.SubtitlePath, "\\", "/")
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

		// Mark as transcoded
		session.mu.Lock()
		session.segments[segmentNum] = true
		session.mu.Unlock()

		logger.Info("Segment transcoded", "session", session.ID, "segment", segmentNum)
	}

	// Serve the segment
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache segments

	http.ServeFile(w, r, segmentPath)
} // Cleanup removes old sessions
func (m *HLSManagerManual) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		os.RemoveAll(session.OutputDir)
		delete(m.sessions, id)
	}
}
