package stream

import (
	"context"
	"wails-cast/pkg/mix"
)

// StreamHandler defines the interface for handling media streams
type StreamHandler interface {
	// ServeManifestPlaylist generates the manifest playlist content
	// Returns the playlist content as string and any error encountered
	ServeManifestPlaylist(ctx context.Context) (string, error)

	// ServeTrackPlaylist generates video or audio track playlists
	// trackType should be "video" or "audio"
	// Returns the playlist content as string and any error encountered
	ServeTrackPlaylist(ctx context.Context, trackType string) (string, error)

	// ServeSegment generates a media segment
	// Returns the file path to serve and any error encountered
	ServeSegment(ctx context.Context, trackType string, segmentIndex int) (*mix.FileOrBuffer, error)

	// ServeSubtitles returns the subtitle file in WebVTT format
	// Returns the subtitle content as string and any error encountered
	ServeSubtitles(ctx context.Context) (string, error)
}
