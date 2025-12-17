package ffmpeg

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
	"wails-cast/pkg/hls"
	"wails-cast/pkg/logger"
	"wails-cast/pkg/mix"

	"wails-cast/pkg/subtitles"

	"github.com/pkg/errors"
)

// TranscodeOptions contains options for transcoding
type TranscodeOptions struct {
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
func TranscodeSegment(ctx context.Context, input *mix.FileOrBuffer, target *mix.TargetFileOrBuffer, opts *TranscodeOptions) (*mix.FileOrBuffer, error) {
	// Build ffmpeg arguments
	args, err := buildTranscodeArgs(input.ToPipe(), target.ToPipe(), opts)
	if err != nil {
		return nil, err
	}

	// log the call
	fmt.Printf(">>>> ffmpeg %s\n\n", strings.Join(args, " "))
	initPaths(false)
	return ffmpeg(ctx, input, target, args)
}

func ffmpeg(ctx context.Context, input *mix.FileOrBuffer, output *mix.TargetFileOrBuffer, args []string) (*mix.FileOrBuffer, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if input.IsBuffer {
		cmd.Stdin = bytes.NewReader(input.Buffer)
	}

	outputData, err := cmd.Output()
	if err != nil {
		if !output.IsBuffer {
			os.Remove(output.FilePath)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fmt.Println(stderr.String())
		return nil, errors.Wrapf(err, "%s", stderr.String())
	}

	if output.IsBuffer {
		return mix.Buffer(outputData), nil
	} else {
		return output.ToOutput(), nil
	}
}

// buildTranscodeArgs builds ffmpeg arguments based on options
func buildTranscodeArgs(inputPath string, outputPath string, opts *TranscodeOptions) ([]string, error) {
	args := []string{"-y"}

	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
	}
	if opts.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", opts.Duration))
	}

	// Input file
	args = append(args, "-i", inputPath)

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

	subtitleFilter, err := buildSubtitleFilter(filepath.Dir(outputPath), opts.Subtitle, inputPath)
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
	return append(args, outputPath), nil
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
		err := ExtractSubtitle(videoPath, embeddedIndex, extractPath)
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

func ExportEmbeddedSubtitles(videoPath string) error {
	if strings.HasPrefix(videoPath, "http://") || strings.HasPrefix(videoPath, "https://") {
		return fmt.Errorf("cannot export subtitles from remote URLs")
	}

	trackInfo, err := GetMediaTrackInfo(videoPath)
	if err != nil {
		return fmt.Errorf("failed to get track info: %w", err)
	}

	if len(trackInfo.SubtitleTracks) == 0 {
		return fmt.Errorf("no embedded subtitles found")
	}

	baseDir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	subtitleDir := filepath.Join(baseDir, baseName)

	// Create subtitle directory
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return fmt.Errorf("failed to create subtitle directory: %w", err)
	}

	for _, sub := range trackInfo.SubtitleTracks {
		name := sub.Name
		if name == "" {
			name = sub.Language
		}
		if name == "" {
			name = fmt.Sprintf("track%d", sub.Index)
		}

		outputFile := filepath.Join(subtitleDir, fmt.Sprintf("%s.vtt", name))

		// Use ffmpeg to extract subtitle
		err := ExtractSubtitle(videoPath, sub.Index, outputFile)
		if err != nil {
			logger.Logger.Warn("Failed to export subtitle", "index", sub.Index, "language", name, "error", err)
			continue
		}
	}

	return nil
}

func ExtractSubtitle(videoPath string, subIndex int, outputFile string) error {
	args := []string{
		"-i", videoPath,
		"-map", fmt.Sprintf("0:s:%d", subIndex),
		"-f", "webvtt",
		"-y",
		outputFile,
	}
	_, err := ffmpeg(context.Background(), mix.File(videoPath), mix.FileTarget(outputFile), args)
	return err
}

// GetMediaTrackInfo gets all track information for a media file using ffprobe
func GetMediaTrackInfo(mediaPath string) (*hls.ManifestPlaylist, error) {
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

	info := &hls.ManifestPlaylist{
		VideoTracks:    make([]hls.VideoTrack, 0),
		AudioTracks:    make([]hls.AudioTrack, 0),
		SubtitleTracks: make([]hls.SubtitleTrack, 0),
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
			info.VideoTracks = append(info.VideoTracks, hls.VideoTrack{
				Index:      videoIdx,
				Codecs:     stream.CodecName,
				Resolution: resolution,
			})
			videoIdx++
		case "audio":
			info.AudioTracks = append(info.AudioTracks, hls.AudioTrack{
				Index:    audioIdx,
				Language: stream.Tags.Language,
			})
			audioIdx++
		case "subtitle":
			info.SubtitleTracks = append(info.SubtitleTracks, hls.SubtitleTrack{
				Index:    subtitleIdx,
				Language: stream.Tags.Language,
				Name:     stream.Tags.Title,
			})
			subtitleIdx++
		}
	}

	return info, nil
}
