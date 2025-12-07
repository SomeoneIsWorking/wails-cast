package options

// CastOptions holds options for casting media
type CastOptions struct {
	SubtitlePath   string
	SubtitleBurnIn bool
	VideoTrack     int
	AudioTrack     int
	Bitrate        string
}
