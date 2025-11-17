package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HLSManagerAuto manages HLS streaming sessions (auto mode - FFmpeg generates all segments upfront)
type HLSManagerAuto struct {
	sessions map[string]*HLSSessionAuto
	mu       sync.RWMutex
	baseDir  string
	localIP  string
}

// HLSSessionAuto represents an active HLS streaming session (auto mode)
type HLSSessionAuto struct {
	ID           string
	VideoPath    string
	SubtitlePath string
	OutputDir    string
	PlaylistPath string
	ready        chan struct{} // Signal when first segment is ready
	cmd          *exec.Cmd
	cancel       context.CancelFunc
}

// NewHLSManagerAuto creates a new HLS manager (auto mode)
func NewHLSManagerAuto(localIP string) *HLSManagerAuto {
	baseDir := filepath.Join(os.TempDir(), "wails-cast-hls")
	os.MkdirAll(baseDir, 0755)

	return &HLSManagerAuto{
		sessions: make(map[string]*HLSSessionAuto),
		baseDir:  baseDir,
		localIP:  localIP,
	}
} // GetOrCreateSession gets existing session or creates new one
func (m *HLSManagerAuto) GetOrCreateSession(videoPath, subtitlePath string) interface{} {
	// Use video path + seek time as session key
	sessionID := fmt.Sprintf("%s", filepath.Base(videoPath))

	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session
	}

	// Create new session
	outputDir := filepath.Join(m.baseDir, sessionID)
	os.MkdirAll(outputDir, 0755)

	session := &HLSSessionAuto{
		ID:           sessionID,
		VideoPath:    videoPath,
		SubtitlePath: subtitlePath,
		OutputDir:    outputDir,
		PlaylistPath: filepath.Join(outputDir, "playlist.m3u8"),
		ready:        make(chan struct{}),
	}

	m.sessions[sessionID] = session

	// Start transcoding
	go m.startTranscode(session)

	return session
}

// startTranscode starts FFmpeg to generate HLS segments
func (m *HLSManagerAuto) startTranscode(session *HLSSessionAuto) {
	logger.Info("Starting HLS transcode", "session", session.ID, "video", session.VideoPath)

	ctx, cancel := context.WithCancel(context.Background())
	session.cancel = cancel

	args := []string{}

	args = append(args, "-i", session.VideoPath)

	// Video encoding
	args = append(args,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-pix_fmt", "yuv420p",
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

	// HLS output
	segmentPath := filepath.Join(session.OutputDir, "segment%d.ts")
	baseURL := fmt.Sprintf("http://%s:8888/", m.localIP)
	args = append(args,
		"-f", "hls",
		"-hls_time", "4",
		"-hls_list_size", "0",
		"-hls_segment_filename", segmentPath,
		"-hls_base_url", baseURL,
		session.PlaylistPath,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	session.cmd = cmd

	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start FFmpeg", "error", err)
		close(session.ready)
		return
	}

	// Wait for first segment to be ready
	go func() {
		for i := 0; i < 50; i++ {
			if _, err := os.Stat(session.PlaylistPath); err == nil {
				close(session.ready)
				logger.Info("HLS session ready", "session", session.ID)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		close(session.ready)
	}()

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		logger.Error("FFmpeg error", "error", err)
	}
}

// ServePlaylist serves the HLS playlist
func (m *HLSManagerAuto) ServePlaylist(w http.ResponseWriter, r *http.Request, session interface{}) {
	s := session.(*HLSSessionAuto)
	// Wait for session to be ready
	<-s.ready

	if _, err := os.Stat(s.PlaylistPath); err != nil {
		http.Error(w, "Playlist not ready", http.StatusNotFound)
		return
	}

	// Read FFmpeg's playlist
	content, err := os.ReadFile(s.PlaylistPath)
	if err != nil {
		http.Error(w, "Failed to read playlist", http.StatusInternalServerError)
		return
	}

	// Convert to string and remove VOD markers to make it appear as live stream (no duration)
	playlistContent := string(content)

	// Set CORS headers for Chromecast
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")

	w.Write([]byte(playlistContent))
} // ServeSegment serves an HLS segment
func (m *HLSManagerAuto) ServeSegment(w http.ResponseWriter, r *http.Request, session interface{}, segmentName string) {
	s := session.(*HLSSessionAuto)
	segmentPath := filepath.Join(s.OutputDir, segmentName)

	// Wait for segment file
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(segmentPath); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if _, err := os.Stat(segmentPath); err != nil {
		http.Error(w, "Segment not found", http.StatusNotFound)
		return
	}

	// Set CORS headers for Chromecast
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	http.ServeFile(w, r, segmentPath)
}

// Cleanup removes old sessions
func (m *HLSManagerAuto) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		if session.cancel != nil {
			session.cancel()
		}
		os.RemoveAll(session.OutputDir)
		delete(m.sessions, id)
	}
}
