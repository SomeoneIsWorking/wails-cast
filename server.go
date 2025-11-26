package main

import (
	"fmt"
	"net/http"
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
	streamHandler  stream.StreamHandler
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

// SetHandler sets the stream handler
func (s *Server) SetHandler(handler stream.StreamHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.streamHandler != nil {
		s.streamHandler.Cleanup()
	}
	s.streamHandler = handler
	logger.Info("Server handler set")
}

// SetSubtitlePath sets the subtitle file
func (s *Server) SetSubtitlePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subtitlePath = path

	// If currently serving local media, update the handler
	if s.currentMedia != "" && s.currentMedia != "remote" {
		if s.streamHandler != nil {
			s.streamHandler.Cleanup()
		}
		// We need options to recreate the handler.
		// Since we don't store full options, we might lose track selection if we just re-init.
		// Ideally, we should update the existing handler or store options.
		// For now, let's assume default options or try to preserve what we can.
		// But LocalHandler stores options now.

		// Better approach: Cast LocalHandler and update its options.
		if handler, ok := s.streamHandler.(*stream.LocalHandler); ok {
			handler.Options.SubtitlePath = path
			// Also update internal subtitle path if needed
			handler.SubtitlePath = path
			// We don't need to recreate the handler, just update it.
			// But LocalHandler might need to clear cache or something.
			// LocalHandler.ServeSegment checks if manifest matches subtitle path.
			// So updating the struct field should be enough.
			return
		}

		// If not LocalHandler (or if we want to be safe), we recreate.
		// But we don't have options here.
		// Let's just log warning if we can't update.
		logger.Warn("Could not update subtitle path on current handler")
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

	if s.streamHandler != nil {
		s.streamHandler.Cleanup()
	}
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleRequest routes requests to appropriate handlers
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	handler := s.streamHandler
	subtitlePath := s.subtitlePath
	s.mu.RUnlock()

	logger.Info("HTTP request", "path", r.URL.Path, "method", r.Method)

	if handler == nil {
		logger.Warn("Request rejected: no media handler set")
		http.Error(w, "No media handler set", http.StatusNotFound)
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

		handler.ServePlaylist(w, r)
		return
	}

	// Handle HLS segment request
	if strings.HasSuffix(path, ".ts") || strings.Contains(path, "/segment/") {
		// Briefly inhibit sleep on streaming requests
		if s.sleepInhibitor != nil {
			s.sleepInhibitor.Refresh(30 * time.Second)
		}

		handler.ServeSegment(w, r)
		return
	}

	// Handle debug log (for remote)
	if path == "/debug/log" {
		// We might need to cast to RemoteHandler to access handleDebugLog if it's not in the interface
		// Or we can add HandleDebugLog to the interface?
		// For now, let's check if it's a RemoteHandler
		if rh, ok := handler.(*stream.RemoteHandler); ok {
			rh.HandleDebugLog(w, r)
		}
		// Actually, I can just not handle it here if not needed, or export it.
		// I will export it in debug_handler.go as HandleDebugLog.
	}

	http.NotFound(w, r)
}
