package hls

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"wails-cast/pkg/mediainfo"
)

// RunFFmpeg runs ffmpeg with the given arguments
func RunFFmpeg(args ...string) error {
	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println(stderr.String())
		return err
	}
	return nil
}

// TranscodeOptions contains options for transcoding
type TranscodeOptions struct {
	InputPath  string
	OutputPath string
	StartTime  float64
	Duration   int
	Subtitle   string
	Bitrate    string
}

// TranscodeSegment transcodes a segment with optional 100ms wait to avoid wasted work during rapid seeking
func TranscodeSegment(ctx context.Context, opts TranscodeOptions) error {
	// Build ffmpeg arguments
	args, err := buildTranscodeArgs(opts)
	if err != nil {
		return err
	}

	// log the call
	fmt.Printf(">>>> ffmpeg %s\n\n", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
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
func buildTranscodeArgs(opts TranscodeOptions) ([]string, error) {
	args := []string{"-y"}

	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
	}
	if opts.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", opts.Duration))
	}

	// Input file
	args = append(args, "-i", opts.InputPath)

	args = append(args,
		"-c:v", "h264_videotoolbox",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "96k",
		"-ac", "2",
		"-f", "mpegts",
		"-copyts",
	)

	if opts.Bitrate != "" {
		args = append(args, "-b:v", opts.Bitrate)
	}

	filterStr, err := buildSubtitleFilter(opts.OutputPath, opts.Subtitle, opts.InputPath)
	if err != nil {
		return nil, err
	}
	if filterStr != "" {
		args = append(args, "-vf", filterStr)
	}

	// Output file
	return append(args, opts.OutputPath), nil
}

// buildSubtitleFilter builds the subtitle filter string for ffmpeg
func buildSubtitleFilter(outputDir string, subtitle string, videoPath string) (string, error) {
	if path, found := strings.CutPrefix(subtitle, "external:"); found {
		err := ensureSubtitleLink(outputDir, path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("subtitles='%s':force_style='FontSize=24'", filepath.Join(outputDir, "input_subtitle")), nil
	}
	if path, found := strings.CutPrefix(subtitle, "embedded:"); found {
		return fmt.Sprintf("subtitles='%s':si=%s:force_style='FontSize=24'", videoPath, path), nil
	}
	return "", nil
}

// ensureSubtitleLink creates a symlink to the subtitle file in the output directory
func ensureSubtitleLink(outputDir string, subtitlePath string) error {
	symlinkPath := filepath.Join(outputDir, "input_subtitle")
	if _, err := os.Lstat(symlinkPath); err != nil {
		return os.Symlink(subtitlePath, symlinkPath)
	}
	return nil
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

// GetMediaTrackInfo gets all track information for a media file using ffprobe
func GetMediaTrackInfo(mediaPath string) (*mediainfo.MediaTrackInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "stream=index,codec_type,codec_name,width,height:stream_tags=language,title",
		"-of", "json",
		mediaPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	info := &mediainfo.MediaTrackInfo{
		VideoTracks:    make([]mediainfo.VideoTrack, 0),
		AudioTracks:    make([]mediainfo.AudioTrack, 0),
		SubtitleTracks: make([]mediainfo.SubtitleTrack, 0),
	}

	videoIdx := 0
	audioIdx := 0
	subtitleIdx := 0

	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			resolution := ""
			if stream.Width > 0 && stream.Height > 0 {
				resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
			}
			info.VideoTracks = append(info.VideoTracks, mediainfo.VideoTrack{
				Index:      videoIdx,
				Codec:      stream.CodecName,
				Resolution: resolution,
			})
			videoIdx++
		case "audio":
			info.AudioTracks = append(info.AudioTracks, mediainfo.AudioTrack{
				Index:    audioIdx,
				Language: stream.Tags.Language,
			})
			audioIdx++
		case "subtitle":
			info.SubtitleTracks = append(info.SubtitleTracks, mediainfo.SubtitleTrack{
				Index:    subtitleIdx,
				Language: stream.Tags.Language,
				Title:    stream.Tags.Title,
			})
			subtitleIdx++
		}
	}

	return info, nil
}
