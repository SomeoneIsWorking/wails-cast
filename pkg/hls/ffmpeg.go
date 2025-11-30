package hls

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TranscodeOptions contains options for transcoding
type TranscodeOptions struct {
	InputPath     string
	OutputPath    string
	StartTime     float64
	Duration      int
	SubtitlePath  string
	SubtitleTrack int // -2 for none, -1 for external, >= 0 for embedded
	BurnIn        bool
	CRF           int
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
			os.Remove(opts.OutputPath)
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

	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
		args = append(args, "-copyts")
	}
	if opts.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", opts.Duration))
	}

	// Input file
	args = append(args, "-i", opts.InputPath)

	args = append(args,
		"-c:v", "h264_videotoolbox",
		"-pix_fmt", "yuv420p",
		"-crf", fmt.Sprintf("%d", opts.CRF),
		"-c:a", "aac",
		"-b:a", "96k",
		"-ac", "2",
		"-f", "mpegts",
		// Timestamp handling for proper segment alignment
		"-avoid_negative_ts", "make_zero",
		"-start_at_zero",
		"-vsync", "cfr",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-g", "48",
	)

	if opts.BurnIn && opts.SubtitleTrack != -2 {
		filterStr := buildSubtitleFilter(opts.OutputPath, opts.SubtitlePath, opts.SubtitleTrack, opts.InputPath)
		if filterStr != "" {
			args = append(args, "-vf", filterStr)
		}
	}

	// Output file
	return append(args, opts.OutputPath)
}

// buildSubtitleFilter builds the subtitle filter string for ffmpeg
func buildSubtitleFilter(outputDir string, subtitlePath string, trackIndex int, videoPath string) string {
	// Check if it's an embedded subtitle track
	if trackIndex >= 0 {
		return fmt.Sprintf("subtitles='%s':si=%d:force_style='FontSize=24'", videoPath, trackIndex)
	}

	ensureSubtitleLink(outputDir, subtitlePath)
	return fmt.Sprintf("subtitles='%s':force_style='FontSize=24'", filepath.Join(outputDir, "input_subtitle"))
}

func ensureSubtitleLink(outputDir string, subtitlePath string) {
	symlinkPath := filepath.Join(outputDir, "input_subtitle")
	if _, err := os.Lstat(symlinkPath); err != nil {
		os.Symlink(subtitlePath, symlinkPath)
	}
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
