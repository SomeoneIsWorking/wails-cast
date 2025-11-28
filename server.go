package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"wails-cast/pkg/stream"
)

// Server is an HTTP server for serving media
type Server struct {
	localIP       string
	port          int
	subtitlePath  string
	streamHandler stream.StreamHandler
	httpServer    *http.Server
	seekTime      int
	mu            sync.RWMutex
}

// NewServer creates a new media server
func NewServer(localIP string, port int) *Server {
	s := &Server{
		localIP: localIP,
		port:    port,
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

	s.streamHandler = handler
	logger.Info("Server handler set")
}

// SetSubtitlePath sets the subtitle file
func (s *Server) SetSubtitlePath(path string) {
	s.mu.Lock()
	s.subtitlePath = path
	defer s.mu.Unlock()

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
	inhibitor.Stop()

	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleRequest routes requests to appropriate handlers
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	logger.Info("HTTP request", "path", path, "method", r.Method)
	s.mu.RLock()
	handler := s.streamHandler
	s.mu.RUnlock()

	if handler == nil {
		http.Error(w, "No media handler set", http.StatusNotFound)
		return
	}

	inhibitor.Refresh(3 * time.Second)

	// Main playlist: /playlist.m3u8 or /media.mp4
	if path == "/playlist.m3u8" {
		handler.ServeMainPlaylist(w, r)
		return
	}
	var trackIndex int

	// Video track playlists: /video_{i}.m3u8
	if _, err := fmt.Sscanf(path, "/video_%d.m3u8", &trackIndex); err == nil {
		handler.ServeTrackPlaylist(w, r, "video", trackIndex)
		return
	}

	// Audio track playlists: /audio_{i}.m3u8
	if _, err := fmt.Sscanf(path, "/audio_%d.m3u8", &trackIndex); err == nil {
		handler.ServeTrackPlaylist(w, r, "audio", trackIndex)
		return
	}

	var segmentIndex int

	// Video segments: /video_{i}/segment_{i}.ts
	if _, err := fmt.Sscanf(path, "/video_%d/segment_%d.ts", &trackIndex, &segmentIndex); err == nil {
		shouldReturn := EnsureRequestDuration(r)
		if shouldReturn {
			return
		}

		handler.ServeSegment(w, r, "video", trackIndex, segmentIndex)
		return
	}

	// Audio segments: /audio_{i}/segment_{i}.ts
	if _, err := fmt.Sscanf(path, "/audio_%d/segment_%d.ts", &trackIndex, &segmentIndex); err == nil {
		shouldReturn := EnsureRequestDuration(r)
		if shouldReturn {
			return
		}

		handler.ServeSegment(w, r, "audio", trackIndex, segmentIndex)
		return
	}

	// Debug log
	if path == "/debug/log" {
		if rh, ok := handler.(*stream.RemoteHandler); ok {
			rh.HandleDebugLog(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func EnsureRequestDuration(r *http.Request) bool {
	select {
	case <-r.Context().Done():
		// Client disconnected/cancelled - don't transcode
		return true
	case <-time.After(100 * time.Millisecond):
		// Connection still alive, proceed with transcode
	}
	return false
}
