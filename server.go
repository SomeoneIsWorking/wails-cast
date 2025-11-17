package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

// Server handles HTTP media streaming
type Server struct {
	port         int
	httpServer   *http.Server
	hlsManager   *HLSManagerManual
	currentMedia string
	subtitlePath string
	seekTime     int
	localIP      string
	mu           sync.RWMutex
}

// NewServer creates a new media server
func NewServer(port int, localIP string) *Server {
	s := &Server{
		port:       port,
		localIP:    localIP,
		hlsManager: NewHLSManagerManual(localIP), // Using manual mode by default
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return s
}

// SetCurrentMedia sets the media file to serve
func (s *Server) SetCurrentMedia(filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentMedia = filePath
	logger.Info("Server now serving", "file", filePath)
}

// SetSubtitlePath sets the subtitle file
func (s *Server) SetSubtitlePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subtitlePath = path
	if path != "" {
		logger.Info("Subtitle path set", "path", path)
	}
}

// SetSeekTime sets the seek position
func (s *Server) SetSeekTime(seconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seekTime = seconds
}

// Start starts the HTTP server
func (s *Server) Start() error {
	logger.Info("Starting media server", "port", s.port)
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	s.hlsManager.Cleanup()
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleRequest routes requests to appropriate handlers
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	videoPath := s.currentMedia
	subtitlePath := s.subtitlePath
	seekTime := s.seekTime
	s.mu.RUnlock()

	logger.Info("HTTP request", "path", r.URL.Path, "method", r.Method)

	if videoPath == "" {
		http.Error(w, "No media file set", http.StatusNotFound)
		return
	}

	path := r.URL.Path

	// Handle HLS playlist request
	if strings.HasSuffix(path, ".m3u8") || path == "/media.mp4" {
		session := s.hlsManager.GetOrCreateSession(videoPath, subtitlePath, seekTime)
		s.hlsManager.ServePlaylist(w, r, session)
		return
	}

	// Handle HLS segment request
	if strings.HasSuffix(path, ".ts") {
		segmentName := filepath.Base(path)
		session := s.hlsManager.GetOrCreateSession(videoPath, subtitlePath, seekTime)
		s.hlsManager.ServeSegment(w, r, session, segmentName)
		return
	}

	// Direct file serving (no transcoding needed)
	ext := strings.ToLower(filepath.Ext(videoPath))
	if ext == ".mp4" && subtitlePath == "" {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")
		http.ServeFile(w, r, videoPath)
		return
	}

	// Default to HLS for everything else
	http.Redirect(w, r, "/media.m3u8", http.StatusFound)
}
