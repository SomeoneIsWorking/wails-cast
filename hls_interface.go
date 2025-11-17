package main

import "net/http"

// HLSMode defines the transcoding mode
type HLSMode int

const (
	HLSModeAuto   HLSMode = iota // Auto: FFmpeg generates all segments upfront
	HLSModeManual                // Manual: Segments generated on-demand
)

// HLSProvider interface for HLS streaming implementations
type HLSProvider interface {
	GetOrCreateSession(videoPath, subtitlePath string) interface{}
	ServePlaylist(w http.ResponseWriter, r *http.Request, session interface{})
	ServeSegment(w http.ResponseWriter, r *http.Request, session interface{}, segmentName string)
	Cleanup()
}
