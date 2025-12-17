package stream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-cast/pkg/ffmpeg"
	"wails-cast/pkg/filehelper"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/mix"
	"wails-cast/pkg/options"
	"wails-cast/pkg/subtitles"
	"wails-cast/pkg/urlhelper"
)

// LocalHandler represents a local file HLS streaming server
type LocalHandler struct {
	VideoPath        string
	Options          options.StreamOptions
	Duration         float64
	SegmentSize      int
	StorageDirectory string
}

// NewLocalHandler creates a new local HLS handler
func NewLocalHandler(videoPath string, options options.StreamOptions) *LocalHandler {
	duration, err := ffmpeg.GetVideoDuration(videoPath)
	if err != nil {
		duration = 0
	}

	return &LocalHandler{
		VideoPath:   videoPath,
		Options:     options,
		Duration:    duration,
		SegmentSize: 8,
	}
}

// ServeManifestPlaylist generates the manifest HLS playlist
func (s *LocalHandler) ServeManifestPlaylist(ctx context.Context) (string, error) {
	manifestPlaylist := &hls.ManifestPlaylist{
		Version: 3,
		VideoTracks: []hls.VideoTrack{
			{
				Index:  0,
				Codecs: "avc1.4d401f,mp4a.40.2",
				URI:    urlhelper.Parse("video.m3u8"),
			},
		},
	}

	// Add subtitle track if available and not burned in
	if s.Options.Subtitle.Path != "none" && !s.Options.Subtitle.BurnIn {
		manifestPlaylist.SubtitleTracks = []hls.SubtitleTrack{
			{
				URI:        urlhelper.Parse("subtitles.vtt"),
				GroupID:    "subs",
				Name:       "Subtitles",
				Language:   "en",
				Default:    true,
				Autoselect: true,
				Forced:     false,
				Index:      0,
			},
		}
		manifestPlaylist.VideoTracks[0].Subtitles = "subs"
	}

	return manifestPlaylist.Generate(), nil
}

// ServeTrackPlaylist generates video or audio track playlists
func (s *LocalHandler) ServeTrackPlaylist(ctx context.Context, trackType string) (string, error) {
	trackPlaylist := &hls.TrackPlaylist{
		Version:        3,
		TargetDuration: s.SegmentSize,
		MediaSequence:  0,
		Segments:       make([]*hls.Segment, 0),
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

		segment := &hls.Segment{
			Duration:        segmentDuration,
			Title:           "",
			URI:             urlhelper.UPrintf("/%s/segment_%d.ts", trackType, i),
			ProgramDateTime: segmentTime.Format(time.RFC3339Nano),
		}
		trackPlaylist.Segments = append(trackPlaylist.Segments, segment)
		cumulativeTime += segmentDuration
	}
	return trackPlaylist.Generate(), nil
}

// ServeSegment transcodes and returns the segment file path
func (s *LocalHandler) ServeSegment(ctx context.Context, trackType string, segmentIndex int) (*mix.FileOrBuffer, error) {
	segmentName := fmt.Sprintf("segment_%d.ts", segmentIndex)
	segmentPath := s.cacheFile(segmentName)
	segmentDuration := float64(s.SegmentSize)
	startTime := float64(segmentIndex * s.SegmentSize)
	if startTime+segmentDuration > s.Duration {
		segmentDuration = s.Duration - startTime
	}

	if s.Options.NoTranscodeCache {
		return s.transcodeSegment(ctx, mix.BufferTarget(), startTime)
	}

	manifest, err := ffmpeg.LoadSegmentManifest(segmentPath + ".json")

	needsRegeneration := err != nil ||
		!ffmpeg.ManifestMatches(manifest, s.Options, s.SegmentSize) ||
		!filehelper.Exists(segmentPath)

	if needsRegeneration {
		buffer, err := s.transcodeSegment(ctx, mix.FileTarget(segmentPath), startTime)
		if err != nil {
			return nil, fmt.Errorf("transcode failed: %w", err)
		}
		return buffer, nil
	}

	return mix.File(segmentPath), nil
}

func (s *LocalHandler) transcodeSegment(ctx context.Context, target *mix.TargetFileOrBuffer, startTime float64) (*mix.FileOrBuffer, error) {
	ensureSymlink(s.VideoPath, folders.Video(s.VideoPath))
	var subtitle *ffmpeg.SubtitleTranscodeOptions = nil

	if s.Options.Subtitle.BurnIn {
		subtitle = &ffmpeg.SubtitleTranscodeOptions{}
	}

	opts := &ffmpeg.TranscodeOptions{
		StartTime:      startTime,
		Duration:       s.SegmentSize,
		Subtitle:       subtitle,
		MaxOutputWidth: s.Options.MaxOutputWidth,
		Bitrate:        s.Options.Bitrate,
	}

	input := mix.File(filepath.Join(folders.Video(s.VideoPath), "input_video"))
	output, err := ffmpeg.TranscodeSegment(ctx, input, target, opts)

	if err != nil {
		return nil, err
	}

	return output, err
}

func ensureSymlink(filePath string, folder string) {
	linkPath := filepath.Join(folder, "input_video")
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		os.Symlink(filePath, linkPath)
	}
}

func (s *LocalHandler) cacheFile(segmentName string) string {
	return filepath.Join(folders.Video(s.VideoPath), segmentName)
}

// ServeSubtitles returns the subtitle file in WebVTT format
func (this *LocalHandler) ServeSubtitles(ctx context.Context) (string, error) {
	if this.Options.Subtitle.Path == "none" || this.Options.Subtitle.BurnIn {
		return "", fmt.Errorf("no external subtitles available")
	}

	subtitlePath := this.Options.Subtitle.Path

	// Handle external subtitle format
	if path, found := strings.CutPrefix(subtitlePath, "external:"); found {
		subtitlePath = path
	} else if embeddedIndex, err := ffmpeg.GetEmbeddedIndex(subtitlePath); err == nil {
		cachedPath := filepath.Join(this.StorageDirectory, fmt.Sprintf("subtitle_%d.vtt", embeddedIndex))
		ffmpeg.ExtractSubtitle(this.VideoPath, embeddedIndex, cachedPath)
		subtitlePath = cachedPath
	}

	// Read subtitle file
	data, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Check if it's already WebVTT
	content := string(data)

	// Otherwise parse and convert to WebVTT
	subtitles, err := subtitles.Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse subtitle file: %w", err)
	}

	// Apply IgnoreClosedCaptions option if requested
	if this.Options.Subtitle.IgnoreClosedCaptions {
		subtitles = subtitles.RemoveClosedCaptions()
	}

	return subtitles.ToWebVTTString(), nil
}
