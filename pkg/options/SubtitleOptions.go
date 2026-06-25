package options

type SubtitleCastOptions struct {
	Path                 string
	BurnIn               bool
	FontSize             int
	IgnoreClosedCaptions bool

	// DelaySeconds shifts subtitle timing. Positive = subtitles appear later,
	// negative = earlier. Applied by shifting served VTT cue timestamps.
	DelaySeconds float64
	// Bold / Italic style the rendered subtitles. For external (re-sent) VTT
	// this is applied via libass when burning in; for burn-in it is passed to
	// the ffmpeg subtitles force_style directive.
	Bold   bool
	Italic bool
}
