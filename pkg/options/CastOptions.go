package options

type CastOptions struct {
	Stream         StreamOptions
	Debug          bool // true to enable debug mode
	NoCastJustHost bool // true to only host the stream without casting
}
