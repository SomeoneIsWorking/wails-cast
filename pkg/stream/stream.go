package stream

import "net/http"

// StreamOptions holds options for streaming
type StreamOptions struct {
	SubtitlePath  string
	SubtitleTrack int
	VideoTrack    int
	AudioTrack    int
	BurnIn        bool
	Quality       string
}

// StreamHandler defines the interface for handling media streams
type StreamHandler interface {
	// ServePlaylist serves the master or media playlist
	ServePlaylist(w http.ResponseWriter, r *http.Request)

	// ServeSegment serves a media segment
	ServeSegment(w http.ResponseWriter, r *http.Request)

	// Cleanup cleans up any resources used by the handler
	Cleanup()
}
