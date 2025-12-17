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
	"wails-cast/pkg/filehelper"
	"wails-cast/pkg/mediainfo"
	"wails-cast/pkg/subtitles"

	"github.com/pkg/errors"
)

// TranscodeOptions contains options for transcoding
type TranscodeOptions struct {
	InputPath      string
	StartTime      float64
	Duration       int
	Bitrate        string
	MaxOutputWidth int
	Subtitle       *SubtitleTranscodeOptions
}

type SubtitleTranscodeOptions struct {
	Path                 string
	FontSize             int
	IgnoreClosedCaptions bool
}

// TranscodeSegment transcodes a segment with optional 100ms wait to avoid wasted work during rapid seeking
func TranscodeSegment(ctx context.Context, opts *TranscodeOptions, outputPath string) ([]byte, error) {
	// Build ffmpeg arguments
	args, err := buildTranscodeArgs(opts, outputPath)
	if err != nil {
		return nil, err
	}

	// log the call
	fmt.Printf(">>>> ffmpeg %s\n\n", strings.Join(args, " "))
	initPaths(false)
	return ffmpeg(ctx, args, outputPath)
}

func ffmpeg(ctx context.Context, args []string, outputPath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, append(args, outputPath)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if outputPath != "pipe:1" {
			os.Remove(outputPath)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fmt.Println(stderr.String())
		return nil, errors.Wrapf(err, "%s", stderr.String())
	}

	return output, nil
}

// buildTranscodeArgs builds ffmpeg arguments based on options
func buildTranscodeArgs(opts *TranscodeOptions, outputPath string) ([]string, error) {
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

	var filterStr []string

	if opts.MaxOutputWidth > 0 {
		filterStr = append(filterStr, fmt.Sprintf("scale='min(%d,iw)':'-2':'force_original_aspect_ratio=decrease'", opts.MaxOutputWidth))
	}

	subtitleFilter, err := buildSubtitleFilter(filepath.Dir(outputPath), opts.Subtitle, opts.InputPath)
	if err != nil {
		return nil, err
	}

	if subtitleFilter != "" {
		// Combine the max resolution filter and the subtitle filter
		filterStr = append(filterStr, subtitleFilter)
	}

	if len(filterStr) > 0 {
		args = append(args, "-vf", strings.Join(filterStr, ","))
	}
	// Output file
	return args, nil
}

// buildSubtitleFilter builds the subtitle filter string for ffmpeg
func buildSubtitleFilter(outputDir string, subtitle *SubtitleTranscodeOptions, videoPath string) (string, error) {
	if subtitle == nil {
		return "", nil
	}
	extractPath := filepath.Join(outputDir, "input_subtitle.vtt")
	if path, found := strings.CutPrefix(subtitle.Path, "external:"); found {
		err := filehelper.EnsureSymlink(path, extractPath)
		if err != nil {
			return "", err
		}
	}
	embeddedIndex, err := GetEmbeddedIndex(subtitle.Path)
	if err == nil {
		// Extract embedded subtitle to temp file
		_, err := ExtractSubtitles(videoPath, embeddedIndex, extractPath)
		if err != nil {
			return "", err
		}
	}
	if subtitle.IgnoreClosedCaptions {
		path, err := removeClosedCaptions(extractPath)
		if err == nil {
			return fmt.Sprintf("subtitles='%s':force_style='FontSize=%d'", path, subtitle.FontSize), nil
		}

	}
	return fmt.Sprintf("subtitles='%s':force_style='FontSize=%d'", extractPath, subtitle.FontSize), nil
}

func GetEmbeddedIndex(subtitlePath string) (int, error) {
	var index int
	n, err := fmt.Sscanf(subtitlePath, "embedded:%d", &index)
	if n != 1 || err != nil {
		return -1, fmt.Errorf("invalid embedded subtitle format")
	}
	return index, nil
}

func removeClosedCaptions(extractPath string) (string, error) {
	// Load subtitles
	subs, err := subtitles.LoadFromFile(extractPath)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(extractPath)
	cleanPath := filepath.Join(dir, "input_subtitle_nocc.vtt")

	// Remove closed captions
	subs = subs.RemoveClosedCaptions()
	subs.Save(cleanPath)

	return cleanPath, nil
}

// GetVideoDuration gets the duration of a video file using ffprobe
func GetVideoDuration(videoPath string) (float64, error) {
	initPaths(false)
	cmd := exec.Command(ffprobePath,
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

func ExtractSubtitles(videoPath string, subIndex int, outputFile string) (string, error) {
	args := []string{
		"-i", videoPath,
		"-map", fmt.Sprintf("0:s:%d", subIndex),
		"-f", "webvtt",
		"-y",
	}
	bytes, err := ffmpeg(context.Background(), args, outputFile)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// GetMediaTrackInfo gets all track information for a media file using ffprobe
func GetMediaTrackInfo(mediaPath string) (*mediainfo.MediaTrackInfo, error) {
	initPaths(false)
	cmd := exec.Command(ffprobePath,
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
