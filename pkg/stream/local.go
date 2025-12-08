package stream

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/options"
	"wails-cast/pkg/subtitles"
)

// LocalHandler represents a local file HLS streaming server
type LocalHandler struct {
	VideoPath   string
	Options     *options.StreamOptions
	OutputDir   string
	Duration    float64
	SegmentSize int
	LocalIP     string
}

// NewLocalHandler creates a new local HLS handler
func NewLocalHandler(videoPath string, options *options.StreamOptions, localIP string) *LocalHandler {
	duration, err := hls.GetVideoDuration(videoPath)
	if err != nil {
		duration = 0
	}

	hash := md5.Sum([]byte(videoPath))
	cacheKey := hex.EncodeToString(hash[:])
	baseDir := folders.GetCache()
	outputDir := filepath.Join(baseDir, cacheKey)
	hls.EnsureCacheDir(outputDir)

	return &LocalHandler{
		VideoPath:   videoPath,
		Options:     options,
		OutputDir:   outputDir,
		Duration:    duration,
		SegmentSize: 8,
		LocalIP:     localIP,
	}
}

// ServeManifestPlaylist generates the manifest HLS playlist
func (s *LocalHandler) ServeManifestPlaylist(ctx context.Context) (string, error) {
	manifestPlaylist := &hls.ManifestPlaylist{
		Version:        3,
		AudioGroups:    make(map[string][]hls.AudioMedia),
		SubtitleGroups: make(map[string][]hls.SubtitleMedia),
		VideoVariants: []hls.VideoVariant{
			{
				Index:  0,
				Codecs: "avc1.4d401f,mp4a.40.2",
				URI:    "video_0.m3u8",
			},
		},
	}

	// Add subtitle track if available and not burned in
	if s.Options.Subtitle.Path != "none" && !s.Options.Subtitle.BurnIn {
		manifestPlaylist.SubtitleGroups["subs"] = []hls.SubtitleMedia{
			{
				URI:        "subtitles.vtt",
				GroupID:    "subs",
				Name:       "Subtitles",
				Language:   "en",
				Default:    true,
				Autoselect: true,
				Forced:     false,
				Index:      0,
			},
		}
		manifestPlaylist.VideoVariants[0].Subtitles = "subs"
	}

	return manifestPlaylist.Generate(), nil
}

// ServeTrackPlaylist generates video or audio track playlists
func (s *LocalHandler) ServeTrackPlaylist(ctx context.Context, trackType string, trackIndex int) (string, error) {
	trackPlaylist := &hls.TrackPlaylist{
		Version:        3,
		TargetDuration: s.SegmentSize,
		MediaSequence:  0,
		Segments:       make([]hls.Segment, 0),
		EndList:        true,
	}

	numSegments := int(s.Duration) / s.SegmentSize
	if int(s.Duration)%s.SegmentSize != 0 {
		numSegments++
	}

	// Add program date time tags for better sync
	baseTime := time.Now()
	cumulativeTime := 0.0

	for i := 0; i < numSegments; i++ {
		segmentDuration := float64(s.SegmentSize)
		if float64((i+1)*s.SegmentSize) > s.Duration {
			segmentDuration = s.Duration - float64(i*s.SegmentSize)
		}

		// Calculate program date time for this segment
		segmentTime := baseTime.Add(time.Duration(cumulativeTime * float64(time.Second)))

		segment := hls.Segment{
			Duration:        segmentDuration,
			Title:           "",
			URI:             fmt.Sprintf("/%s_0/segment_%d.ts", trackType, i),
			ProgramDateTime: segmentTime.Format(time.RFC3339Nano),
		}
		trackPlaylist.Segments = append(trackPlaylist.Segments, segment)
		cumulativeTime += segmentDuration
	}
	return trackPlaylist.Generate(), nil
}

// ServeSegment transcodes and returns the segment file path
func (s *LocalHandler) ServeSegment(ctx context.Context, trackType string, trackIndex int, segmentIndex int) (string, error) {
	segmentName := fmt.Sprintf("segment_%d.ts", segmentIndex)

	hls.EnsureCacheDir(s.OutputDir)

	segmentPath := hls.GetCachePath(s.OutputDir, segmentName)
	segmentDuration := float64(s.SegmentSize)
	startTime := float64(segmentIndex * s.SegmentSize)
	if startTime+segmentDuration > s.Duration {
		segmentDuration = s.Duration - startTime
	}

	manifest, err := hls.LoadSegmentManifest(segmentPath + ".json")
	needsRegeneration := err != nil || !hls.ManifestMatches(manifest, s.Options, s.SegmentSize)

	if !hls.CacheExists(s.OutputDir, segmentName) || needsRegeneration {
		err := s.transcodeSegment(ctx, segmentPath, startTime)
		if err != nil {
			return "", fmt.Errorf("transcode failed: %w", err)
		}
	}

	return segmentPath, nil
}

func (s *LocalHandler) transcodeSegment(ctx context.Context, segmentPath string, startTime float64) error {
	ensureSymlink(s.VideoPath, s.OutputDir)
	subtitle := ""

	if s.Options.Subtitle.BurnIn {
		subtitle = s.Options.Subtitle.Path
	}

	opts := &hls.TranscodeOptions{
		InputPath:      filepath.Join(s.OutputDir, "input_video"),
		OutputPath:     segmentPath,
		StartTime:      startTime,
		Duration:       s.SegmentSize,
		Subtitle:       subtitle,
		MaxOutputWidth: s.Options.MaxOutputWidth,
		Bitrate:        s.Options.Bitrate,
		FontSize:       s.Options.Subtitle.FontSize,
	}

	err := hls.TranscodeSegment(ctx, opts)
	if err != nil {
		return err
	}

	err = opts.Save(segmentPath + ".json")
	return err
}

func ensureSymlink(filePath string, folder string) {
	linkPath := filepath.Join(folder, "input_video")
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		os.Symlink(filePath, linkPath)
	}
}

// ServeSubtitles returns the subtitle file in WebVTT format
func (s *LocalHandler) ServeSubtitles(ctx context.Context) (string, error) {
	if s.Options.Subtitle.Path == "none" || s.Options.Subtitle.BurnIn {
		return "", fmt.Errorf("no external subtitles available")
	}

	subtitlePath := s.Options.Subtitle.Path

	// Handle external subtitle format
	if path, found := strings.CutPrefix(subtitlePath, "external:"); found {
		subtitlePath = path
	} else if index, found := strings.CutPrefix(subtitlePath, "embedded:"); found {
		// Extract embedded subtitle to cache
		cachedPath := filepath.Join(s.OutputDir, fmt.Sprintf("subtitle_%s.vtt", index))

		// Check if already cached
		if _, err := os.Stat(cachedPath); os.IsNotExist(err) {
			// Extract using ffmpeg
			args := []string{
				"-i", s.VideoPath,
				"-map", fmt.Sprintf("0:s:%s", index),
				"-f", "webvtt",
				"-y",
				cachedPath,
			}

			if err := hls.RunFFmpeg(args...); err != nil {
				return "", fmt.Errorf("failed to extract embedded subtitle: %w", err)
			}
		}

		subtitlePath = cachedPath
	}
	// Otherwise it's a direct path, use as-is

	// Read subtitle file
	data, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Check if it's already WebVTT
	content := string(data)
	if strings.HasPrefix(content, "WEBVTT") {
		return content, nil
	}

	// Otherwise parse and convert to WebVTT
	subtitles, err := subtitles.Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse subtitle file: %w", err)
	}

	return subtitles.ToWebVTTString(), nil
}

// Cleanup removes session files
func (s *LocalHandler) Cleanup() {
	if s.OutputDir != "" {
		os.RemoveAll(s.OutputDir)
	}
}
