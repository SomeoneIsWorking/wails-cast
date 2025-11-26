package hls

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// TranscodeOptions contains options for transcoding
type TranscodeOptions struct {
	InputPath     string
	OutputPath    string
	StartTime     float64
	Duration      int
	SubtitlePath  string
	SubtitleTrack int // -1 for external/none, >= 0 for embedded
	VideoTrack    int // -1 for default
	AudioTrack    int // -1 for default
	BurnIn        bool
	Quality       string // "low", "medium", "high", "original"
	StreamIndex   string // Deprecated: use SubtitleTrack
	IsAudioOnly   bool
	Preset        string
}

// EscapeFFmpegPath escapes special characters in paths that ffmpeg doesn't like
func EscapeFFmpegPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ReplaceAll(path, "[", "\\[")
	path = strings.ReplaceAll(path, "]", "\\]")
	return path
}

// TranscodeSegment transcodes a segment with optional 100ms wait to avoid wasted work during rapid seeking
func TranscodeSegment(ctx context.Context, opts TranscodeOptions) error {
	// Build ffmpeg arguments
	args := buildTranscodeArgs(opts)

	// log the call
	fmt.Printf(">>>> ffmpeg %s\n\n", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check if it was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Println(stderr.String())
		return err
	}

	return nil
}

// buildTranscodeArgs builds ffmpeg arguments based on options
func buildTranscodeArgs(opts TranscodeOptions) []string {
	args := []string{"-y"}

	// Add seek if specified (for local file segments)
	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
	}
	if opts.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", opts.Duration))
	}

	// Input file
	args = append(args, "-i", EscapeFFmpegPath(opts.InputPath))

	// Preserve timestamps for HLS
	args = append(args, "-copyts")

	// Video encoding
	if opts.IsAudioOnly {
		// Audio-only segment
		args = append(args,
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-ac", "2",
		)
	} else {
		// Mixed segment or local file
		args = append(args,
			"-c:v", "libx264",
			"-preset", "fast",
			"-crf", getCRF(opts.Quality),
			"-pix_fmt", "yuv420p",
			"-sc_threshold", "0",
			"-g", "60",
			"-s", "1280x720",
			"-r", "30",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "48000",
			"-ac", "2",
		)

		// Add subtitles if provided (for local files)
		if opts.BurnIn && opts.SubtitlePath != "" {
			filterStr := buildSubtitleFilter(opts.SubtitlePath, opts.SubtitleTrack, opts.InputPath)
			if filterStr != "" {
				args = append(args, "-vf", filterStr)
			}
		}

		// Audio encoding
		args = append(args,
			"-c:a", "aac",
			"-b:a", "96k",
			"-ac", "2",
		)
	}

	// Output format
	args = append(args,
		"-f", "mpegts",
		"-muxdelay", "0",
		"-muxpreload", "0",
		opts.OutputPath,
	)

	return args
}

// getCRF returns the CRF value based on quality setting
func getCRF(quality string) string {
	switch quality {
	case "low":
		return "28"
	case "medium":
		return "25"
	case "high":
		return "23"
	case "original":
		return "18"
	default:
		return "28"
	}
}

// buildSubtitleFilter builds the subtitle filter string for ffmpeg
func buildSubtitleFilter(subtitlePath string, trackIndex int, videoPath string) string {
	// Check if it's an embedded subtitle track
	if trackIndex >= 0 {
		return fmt.Sprintf("subtitles='%s':si=%d:force_style='FontSize=24'",
			EscapeFFmpegPath(videoPath), trackIndex)
	}

	// External subtitle file
	if subtitlePath != "" {
		return fmt.Sprintf("subtitles='%s':force_style='FontSize=24'",
			EscapeFFmpegPath(subtitlePath))
	}

	return ""
}

// getPreset returns the ffmpeg preset, defaulting to "veryfast"
func getPreset(preset string) string {
	if preset == "" {
		return "veryfast"
	}
	return preset
}

// GetVideoDuration gets the duration of a video file using ffprobe
func GetVideoDuration(videoPath string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var duration float64
	_, err = fmt.Sscanf(strings.TrimSpace(out.String()), "%f", &duration)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}
