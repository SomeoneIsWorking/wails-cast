package main

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type Server struct {
	port         int
	httpServer   *http.Server
	mediaManager *MediaManager
}

func NewServer(port int, mediaManager *MediaManager) *Server {
	s := &Server{
		port:         port,
		mediaManager: mediaManager,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleMediaRequest)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return s
}

// Start begins listening for media requests
func (s *Server) Start() error {
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			println("Media server error:", err.Error())
		}
	}()
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleMediaRequest serves media files with proper headers
func (s *Server) handleMediaRequest(w http.ResponseWriter, r *http.Request) {
	filePath, err := url.QueryUnescape(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	filePath = strings.TrimPrefix(filePath, "/")
	filePath = filepath.Clean(filePath)

	// Set appropriate content type
	contentType := s.mediaManager.GetContentType(filePath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeFile(w, r, filePath)
}
