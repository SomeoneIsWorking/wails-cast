package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wails-cast/pkg/sleepinhibit"
	"wails-cast/pkg/stream"
)

// Server is an HTTP server for serving media
type Server struct {
	port           int
	localIP        string
	currentMedia   string
	subtitlePath   string
	hlsServer      *stream.LocalHLSServer
	httpServer     *http.Server
	seekTime       int
	sleepInhibitor *sleepinhibit.Inhibitor
	mu             sync.RWMutex
}

// NewServer creates a new media server
func NewServer(port int, localIP string) *Server {
	s := &Server{
		port:           port,
		localIP:        localIP,
		sleepInhibitor: sleepinhibit.NewInhibitor(logger),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0, // No write timeout for streaming
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// SetCurrentMedia sets the media file to serve
func (s *Server) SetCurrentMedia(filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up old session if exists
	if s.hlsServer != nil {
		s.hlsServer.Cleanup()
	}

	s.currentMedia = filePath
	logger.Info("Server now serving", "file", filePath)
}

// SetSubtitlePath sets the subtitle file
func (s *Server) SetSubtitlePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subtitlePath = path

	// Create new session with current media and subtitle
	if s.currentMedia != "" {
		if s.hlsServer != nil {
			s.hlsServer.Cleanup()
		}
		s.hlsServer = stream.NewLocalHLSServer(s.currentMedia, s.subtitlePath, s.localIP)
	}

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
	// Stop sleep inhibition
	if s.sleepInhibitor != nil {
		s.sleepInhibitor.Stop()
	}

	if s.hlsServer != nil {
		s.hlsServer.Cleanup()
	}
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
	s.mu.RUnlock()

	logger.Info("HTTP request", "path", r.URL.Path, "method", r.Method, "videoPath", videoPath)

	if videoPath == "" {
		logger.Warn("Request rejected: no media file set")
		http.Error(w, "No media file set", http.StatusNotFound)
		return
	}

	path := r.URL.Path

	// Handle subtitle request
	if path == "/subtitle.vtt" || strings.HasSuffix(path, ".vtt") || strings.HasSuffix(path, ".srt") {
		if subtitlePath != "" && !strings.Contains(subtitlePath, ":si=") {
			// Serve external subtitle file
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "text/vtt")
			http.ServeFile(w, r, subtitlePath)
			return
		}
		http.NotFound(w, r)
		return
	}

	// Handle HLS playlist request
	if strings.HasSuffix(path, ".m3u8") || path == "/media.mp4" {
		// Briefly inhibit sleep on streaming requests (auto-stops after 30s of inactivity)
		if s.sleepInhibitor != nil {
			s.sleepInhibitor.Refresh(30 * time.Second)
		}

		if s.hlsServer == nil {
			s.hlsServer = stream.NewLocalHLSServer(videoPath, subtitlePath, s.localIP)
		}
		s.hlsServer.ServePlaylist(w, r)
		return
	}

	// Handle HLS segment request
	if strings.HasSuffix(path, ".ts") {
		segmentName := filepath.Base(path)
		logger.Info("Routing to HLS segment handler", "segmentName", segmentName)
		if s.hlsServer == nil {
			s.hlsServer = stream.NewLocalHLSServer(videoPath, subtitlePath, s.localIP)
		}
		s.hlsServer.ServeSegment(w, r, segmentName)
		return
	}

	http.NotFound(w, r)
}
