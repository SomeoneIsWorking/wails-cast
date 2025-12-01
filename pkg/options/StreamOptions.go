package options

// StreamOptions holds options for streaming
type StreamOptions struct {
	Subtitle   SubtitleCastOptions
	VideoTrack int
	AudioTrack int
	CRF        int
}
