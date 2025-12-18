package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"wails-cast/pkg/events"
	"wails-cast/pkg/inhibitor"
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

	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleRequest routes requests to appropriate handlers
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	logger.Info("HTTP request", "URL", r.URL.String(), "method", r.Method)
	s.mu.RLock()
	handler := s.streamHandler
	s.mu.RUnlock()

	if handler == nil {
		http.Error(w, "No media handler set", http.StatusNotFound)
		return
	}

	inhibitor.Refresh()

	// Main playlist: /playlist.m3u8 or /media.mp4
	if path == "/playlist.m3u8" {
		playlist, err := handler.ServeManifestPlaylist(r.Context())
		if err != nil {
			s.handleError(w, r, "Failed to generate manifest playlist", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(playlist))
		return
	}

	// Video track playlists: /video.m3u8
	if path == "/video.m3u8" {
		playlist, err := handler.ServeTrackPlaylist(r.Context(), "video")
		if err != nil {
			s.handleError(w, r, "Failed to generate video track playlist", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(playlist))
		return
	}

	// Audio track playlists: /audio.m3u8
	if path == "/audio.m3u8" {
		playlist, err := handler.ServeTrackPlaylist(r.Context(), "audio")
		if err != nil {
			s.handleError(w, r, "Failed to generate audio track playlist", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(playlist))
		return
	}

	var segmentIndex int

	// Video segments: /video/segment_{i}.ts
	if _, err := fmt.Sscanf(path, "/video/segment_%d.ts", &segmentIndex); err == nil {
		shouldReturn := EnsureRequestDuration(r)
		if shouldReturn {
			return
		}

		buffer, err := handler.ServeSegment(r.Context(), "video", segmentIndex)
		if err != nil {
			s.handleError(w, r, "Failed to generate video segment", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		buffer.Serve(w, r)
		return
	}

	// Audio segments: /audio/segment_{i}.ts
	if _, err := fmt.Sscanf(path, "/audio/segment_%d.ts", &segmentIndex); err == nil {
		shouldReturn := EnsureRequestDuration(r)
		if shouldReturn {
			return
		}

		buffer, err := handler.ServeSegment(r.Context(), "audio", segmentIndex)
		if err != nil {
			s.handleError(w, r, "Failed to generate audio segment", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		buffer.Serve(w, r)
		return
	}

	// Subtitles: /subtitles.vtt
	if path == "/subtitles.vtt" {
		subtitleContent, err := handler.ServeSubtitles(r.Context())
		if err != nil {
			s.handleError(w, r, "Failed to serve subtitles", err)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/vtt")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		subtitleContent.Serve(w, r)
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

// handleError logs an error, writes HTTP response, and emits a Wails event
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, message string, err error) {
	if r.Context().Err() != nil {
		return
	}

	// Log the error
	logger.Error(message, "error", err)

	// Write HTTP error response
	http.Error(w, fmt.Sprintf("%s: %v", message, err), http.StatusInternalServerError)

	// Emit backend event - App will forward to frontend
	errorMessage := fmt.Sprintf("%s: %v", message, err)
	events.Emit("stream:error", map[string]string{
		"message": message,
		"error":   err.Error(),
		"full":    errorMessage,
	})
}
