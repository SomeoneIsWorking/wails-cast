package options

// CastOptions holds options for streaming
type CastOptions struct {
	Subtitle   SubtitleCastOptions
	VideoTrack int
	AudioTrack int
	Bitrate    string
}
