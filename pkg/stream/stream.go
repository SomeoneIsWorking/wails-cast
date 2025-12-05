package stream

import "context"

// StreamHandler defines the interface for handling media streams
type StreamHandler interface {
	// ServeManifestPlaylist generates the manifest playlist content
	// Returns the playlist content as string and any error encountered
	ServeManifestPlaylist(ctx context.Context) (string, error)

	// ServeTrackPlaylist generates video or audio track playlists
	// trackType should be "video" or "audio"
	// trackIndex is the extracted index from the path
	// Returns the playlist content as string and any error encountered
	ServeTrackPlaylist(ctx context.Context, trackType string, trackIndex int) (string, error)

	// ServeSegment generates a media segment
	// Returns the file path to serve and any error encountered
	ServeSegment(ctx context.Context, trackType string, trackIndex int, segmentIndex int) (string, error)
}
