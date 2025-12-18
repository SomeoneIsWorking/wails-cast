package stream

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-cast/pkg/ffmpeg"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/logger"
	"wails-cast/pkg/mix"
	"wails-cast/pkg/options"
	"wails-cast/pkg/remote"
	"wails-cast/pkg/subtitles"
	"wails-cast/pkg/urlhelper"

	"github.com/pkg/errors"
)

// RemoteHandler is a handler that serves HLS manifests and segments
// with captured cookies and headers
type RemoteHandler struct {
	Options      options.StreamOptions
	Manifest     *hls.ManifestPlaylist
	VideoManager *remote.TrackManager
	AudioManager *remote.TrackManager
	CacheDir     string
}

// NewRemoteHandler creates a new HLS handler
func NewRemoteHandler(
	ctx context.Context,
	mediaManager *remote.MediaManager,
	options options.StreamOptions,
	cacheDir string,
) (*RemoteHandler, error) {
	videoManager, err := mediaManager.GetTrack(ctx, "video", options.VideoTrack)
	if err != nil {
		return nil, err
	}
	var audioManager *remote.TrackManager
	if len(mediaManager.Manifest.AudioTracks) > 0 {
		audioManager, err = mediaManager.GetTrack(ctx, "audio", options.AudioTrack)
		if err != nil {
			return nil, err
		}
	}
	return &RemoteHandler{
		Options:      options,
		Manifest:     mediaManager.Manifest,
		VideoManager: videoManager,
		AudioManager: audioManager,
		CacheDir:     cacheDir,
	}, nil
}

// ServeManifestPlaylist generates the manifest playlist
func (p *RemoteHandler) ServeManifestPlaylist(ctx context.Context) (string, error) {
	playlist := &hls.ManifestPlaylist{}

	// Add subtitle track if available and not burned in
	if p.Options.Subtitle.Path != "none" && !p.Options.Subtitle.BurnIn {
		playlist.SubtitleTracks = []hls.SubtitleTrack{
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
	}

	videoVariant := p.Manifest.VideoTracks[p.Options.VideoTrack]
	videoVariant.Resolution = ""
	videoVariant.URI = urlhelper.Parse("/video.m3u8")
	videoVariant.Subtitles = "subs"

	if len(p.Manifest.AudioTracks) > 0 {
		audio := p.Manifest.AudioTracks[p.Options.AudioTrack]
		audio.URI = urlhelper.Parse("/audio.m3u8")
		playlist.AudioTracks = []hls.AudioTrack{audio}
	}

	playlist.VideoTracks = []hls.VideoTrack{videoVariant}
	return playlist.Generate(), nil
}

func (p *RemoteHandler) getTrackManager(trackType string) *remote.TrackManager {
	if trackType == "video" {
		return p.VideoManager
	}
	return p.AudioManager
}

// ServeTrackPlaylist generates video or audio track playlists
func (p *RemoteHandler) ServeTrackPlaylist(ctx context.Context, trackType string) (string, error) {
	trackManager := p.getTrackManager(trackType)
	playlist := *trackManager.Manifest
	playlist.Segments = make([]*hls.Segment, len(trackManager.Manifest.Segments))

	cumulativeTime := 0.0
	baseTime := time.Now()

	for index, segment := range trackManager.Manifest.Segments {
		copy := *segment
		// Add program date time for each segment to help with sync
		segmentTime := baseTime.Add(time.Duration(cumulativeTime * float64(time.Second)))
		copy.ProgramDateTime = segmentTime.Format(time.RFC3339Nano)
		copy.URI = urlhelper.Parse(fmt.Sprintf("/%s/segment_%d.ts", trackType, index))
		playlist.Segments[index] = &copy
		cumulativeTime += segment.Duration
	}

	return playlist.Generate(), nil
}

// ServeSegment proxies segment requests with captured cookies and headers,
// and transcodes them using ffmpeg for compatibility
func (p *RemoteHandler) ServeSegment(ctx context.Context, trackType string, segmentIndex int) (*mix.FileOrBuffer, error) {
	logger.Logger.Info("Proxying request", "type", trackType, "segment", segmentIndex)

	if p.Options.NoTranscodeCache {
		segment, err := p.getTrackManager(trackType).GetSegment(ctx, segmentIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure raw segment exists: %w", err)
		}
		return p.transcodeSegment(ctx, segment, mix.BufferTarget())
	}

	transcodedPath, err := p.ensureSegmentExistsTranscoded(ctx, trackType, segmentIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure transcoded segment exists: %w", err)
	}
	return mix.File(transcodedPath), nil
}

func (p *RemoteHandler) ensureSegmentExistsTranscoded(ctx context.Context, trackType string, segmentIndex int) (string, error) {
	trackIndex := p.getTrackIndex(trackType)
	transcodedPath, err := p.getSegmentPath(trackType, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get transcoded segment path for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}

	// Load manifest and check if segment needs regeneration
	manifest, err := ffmpeg.LoadSegmentManifest(transcodedPath + ".json")
	needsRegeneration := err != nil || !ffmpeg.ManifestMatches(manifest, p.Options, 0)

	if _, err := os.Stat(transcodedPath); err == nil && !needsRegeneration {
		return transcodedPath, nil
	}

	segment, err := p.getTrackManager(trackType).GetSegment(ctx, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to ensure raw segment exists for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}

	_, err = p.transcodeSegment(ctx, segment, mix.FileTarget(transcodedPath))
	if err != nil {
		return "", errors.Wrapf(err, "failed to transcode segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}
	return transcodedPath, nil
}

func (p *RemoteHandler) transcodeSegment(ctx context.Context, input *mix.FileOrBuffer, target *mix.TargetFileOrBuffer) (*mix.FileOrBuffer, error) {
	var subtitle *ffmpeg.SubtitleTranscodeOptions = nil

	if p.Options.Subtitle.BurnIn {
		subtitle = &ffmpeg.SubtitleTranscodeOptions{
			Path:                 p.Options.Subtitle.Path,
			FontSize:             p.Options.Subtitle.FontSize,
			IgnoreClosedCaptions: p.Options.Subtitle.IgnoreClosedCaptions,
		}

		if index, found := strings.CutPrefix(subtitle.Path, "embedded:"); found {
			// For remote streams, embedded just means found on website, in this case, we use the cached file
			subtitle.Path = fmt.Sprintf("external:%s", filepath.Join(p.CacheDir, fmt.Sprintf("subtitle_%s.vtt", index)))
		}
	}

	opts := &ffmpeg.TranscodeOptions{
		StartTime:      0,
		Duration:       0,
		Subtitle:       subtitle,
		Bitrate:        p.Options.Bitrate,
		MaxOutputWidth: p.Options.MaxOutputWidth,
	}
	output, err := ffmpeg.TranscodeSegment(ctx, input, target, opts)
	if err != nil {
		return nil, err
	}
	if !output.IsBuffer {
		err = opts.Save(output.FilePath + ".json")
	}
	return output, err
}

func (p *RemoteHandler) getSegmentPath(trackType string, segmentIndex int) (string, error) {
	trackDir, err := p.getTrackDir(trackType)
	if err != nil {
		return "", err
	}

	localPath := filepath.Join(trackDir, fmt.Sprintf("segment_%d.ts", segmentIndex))
	return localPath, nil
}

func (p *RemoteHandler) getTrackIndex(trackType string) int {
	if trackType == "video" {
		return p.Options.VideoTrack
	}
	return p.Options.AudioTrack
}

func (p *RemoteHandler) getTrackDir(trackType string) (string, error) {
	trackIndex := p.getTrackIndex(trackType)
	trackDir := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, trackIndex))
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		return "", err
	}
	return trackDir, nil
}

// serveFile serves a local file
func (p *RemoteHandler) serveFile(w http.ResponseWriter, path string, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ServeSubtitles returns the subtitle file in WebVTT format
func (p *RemoteHandler) ServeSubtitles(ctx context.Context) (string, error) {
	if p.Options.Subtitle.Path == "none" || p.Options.Subtitle.BurnIn {
		return "", fmt.Errorf("no external subtitles available")
	}

	subtitlePath := p.Options.Subtitle.Path

	// Handle embedded subtitles - they're cached in the cache directory
	if index, found := strings.CutPrefix(subtitlePath, "embedded:"); found {
		subtitlePath = filepath.Join(p.CacheDir, fmt.Sprintf("subtitle_%s.vtt", index))
	} else if path, found := strings.CutPrefix(subtitlePath, "external:"); found {
		subtitlePath = path
	}
	// Otherwise it's a direct path, use as-is

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
	if p.Options.Subtitle.IgnoreClosedCaptions {
		subtitles = subtitles.RemoveClosedCaptions()
	}

	return subtitles.ToWebVTTString(), nil
}
