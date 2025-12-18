package stream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"wails-cast/pkg/ffmpeg"
	"wails-cast/pkg/filehelper"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/mix"
	"wails-cast/pkg/options"
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
		VideoPath:        videoPath,
		Options:          options,
		Duration:         duration,
		SegmentSize:      8,
		StorageDirectory: folders.Video(videoPath),
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
				URI:    urlhelper.ParseFixed("/video.m3u8"),
			},
		},
	}

	// Add subtitle track if available and not burned in
	if s.Options.Subtitle.Path != "none" && !s.Options.Subtitle.BurnIn {
		manifestPlaylist.SubtitleTracks = []hls.SubtitleTrack{
			{
				URI:        urlhelper.ParseFixed("/subtitles.vtt"),
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
	linkPath := filepath.Join(s.StorageDirectory, "input_video")
	err := filehelper.EnsureSymlink(s.VideoPath, linkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create symlink: %w", err)
	}

	var subtitle *ffmpeg.SubtitleTranscodeOptions = nil

	if s.Options.Subtitle.BurnIn {
		path := filepath.Join(s.StorageDirectory, "subtitles.vtt")
		_, err := s.getSubtitles(mix.FileTarget(path))
		if err != nil {
			return nil, fmt.Errorf("failed to get subtitles for burn-in: %w", err)
		}
		subtitle = &ffmpeg.SubtitleTranscodeOptions{
			Path:     path,
			FontSize: s.Options.Subtitle.FontSize,
		}
	}

	opts := &ffmpeg.TranscodeOptions{
		StartTime:      startTime,
		Duration:       s.SegmentSize,
		Subtitle:       subtitle,
		MaxOutputWidth: s.Options.MaxOutputWidth,
		Bitrate:        s.Options.Bitrate,
	}

	output, err := ffmpeg.TranscodeSegment(ctx, mix.File(linkPath), target, opts)

	if err != nil {
		return nil, err
	}

	return output, err
}

func (s *LocalHandler) cacheFile(segmentName string) string {
	return filepath.Join(folders.Video(s.VideoPath), segmentName)
}

// ServeSubtitles returns the subtitle file in WebVTT format
func (this *LocalHandler) ServeSubtitles(ctx context.Context) (*mix.FileOrBuffer, error) {
	if this.Options.Subtitle.Path == "none" || this.Options.Subtitle.BurnIn {
		return nil, fmt.Errorf("no external subtitles available")
	}

	return this.getSubtitles(mix.BufferTarget())
}

func (this *LocalHandler) getSubtitles(target *mix.TargetFileOrBuffer) (*mix.FileOrBuffer, error) {
	subtitles, err := this.readSubtitles(target)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitles: %w", err)
	}
	return ProcessSubtitles(subtitles, target, this.Options.Subtitle.IgnoreClosedCaptions)
}

func (this *LocalHandler) readSubtitles(target *mix.TargetFileOrBuffer) (*mix.FileOrBuffer, error) {
	subtitlePath := this.Options.Subtitle.Path

	// Handle external subtitle format
	if path, found := GetExternalPath(subtitlePath); found {
		if target.IsBuffer {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read subtitle file: %w", err)
			}
			return mix.Buffer(data), nil
		} else {
			filehelper.EnsureSymlink(path, target.FilePath)
			return target.ToOutput(), nil
		}
	} else if index, found := GetEmbeddedIndex(subtitlePath); found {
		return ffmpeg.ExtractSubtitle(this.VideoPath, index, target)
	}

	return nil, fmt.Errorf("unsupported subtitle path format %s", subtitlePath)
}
