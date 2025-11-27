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
	// ServeMainPlaylist serves the main/master playlist (/playlist.m3u8)
	ServeMainPlaylist(w http.ResponseWriter, r *http.Request)

	// ServeTrackPlaylist serves video or audio track playlists (/video_i.m3u8, /audio_i.m3u8)
	// trackType should be "video" or "audio"
	// trackIndex is the extracted index from the path
	ServeTrackPlaylist(w http.ResponseWriter, r *http.Request, trackType string, trackIndex int)

	// ServeSegment serves a media segment
	ServeSegment(w http.ResponseWriter, r *http.Request, trackType string, trackIndex int, segmentIndex int)
}
