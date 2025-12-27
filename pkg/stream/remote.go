package stream

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"wails-cast/pkg/ffmpeg"
	"wails-cast/pkg/filehelper"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/logger"
	"wails-cast/pkg/mix"
	"wails-cast/pkg/options"
	"wails-cast/pkg/remote"
	"wails-cast/pkg/urlhelper"

	"github.com/pkg/errors"
)

// RemoteHandler is a handler that serves HLS manifests and segments
// with captured cookies and headers
type RemoteHandler struct {
	Options          options.StreamOptions
	Manifest         *hls.ManifestPlaylist
	VideoManager     *remote.TrackManager
	AudioManager     *remote.TrackManager
	StorageDirectory string
}

// NewRemoteHandler creates a new HLS handler
func NewRemoteHandler(
	ctx context.Context,
	mediaManager *remote.MediaManager,
	options options.StreamOptions,
	storageDirectory string,
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
		Options:          options,
		Manifest:         mediaManager.Manifest,
		VideoManager:     videoManager,
		AudioManager:     audioManager,
		StorageDirectory: folders.Video(mediaManager.URL),
	}, nil
}

// ServeManifestPlaylist generates the manifest playlist
func (this *RemoteHandler) ServeManifestPlaylist(ctx context.Context) (string, error) {
	playlist := &hls.ManifestPlaylist{}

	videoVariant := this.Manifest.VideoTracks[this.Options.VideoTrack]
	videoVariant.Resolution = ""
	videoVariant.URI = urlhelper.ParseFixed("/video.m3u8")
	videoVariant.Subtitles = ""

	if len(this.Manifest.AudioTracks) > 0 {
		audio := this.Manifest.AudioTracks[this.Options.AudioTrack]
		audio.URI = urlhelper.ParseFixed("/audio.m3u8")
		playlist.AudioTracks = []hls.AudioTrack{audio}
	}

	playlist.VideoTracks = []hls.VideoTrack{videoVariant}
	return playlist.Generate(), nil
}

func (this *RemoteHandler) getTrackManager(trackType string) *remote.TrackManager {
	if trackType == "video" {
		return this.VideoManager
	}
	return this.AudioManager
}

// ServeTrackPlaylist generates video or audio track playlists
func (this *RemoteHandler) ServeTrackPlaylist(ctx context.Context, trackType string) (string, error) {
	trackManager := this.getTrackManager(trackType)
	playlist := *trackManager.Manifest
	playlist.Segments = make([]*hls.Segment, len(trackManager.Manifest.Segments))

	cumulativeTime := 0.0
	baseTime := time.Now()

	for index, segment := range trackManager.Manifest.Segments {
		copy := *segment
		// Add program date time for each segment to help with sync
		segmentTime := baseTime.Add(time.Duration(cumulativeTime * float64(time.Second)))
		copy.ProgramDateTime = segmentTime.Format(time.RFC3339Nano)
		copy.URI = urlhelper.UPrintf("/%s/segment_%d.ts", trackType, index)
		playlist.Segments[index] = &copy
		cumulativeTime += segment.Duration
	}

	return playlist.Generate(), nil
}

// ServeSegment proxies segment requests with captured cookies and headers,
// and transcodes them using ffmpeg for compatibility
func (this *RemoteHandler) ServeSegment(ctx context.Context, trackType string, segmentIndex int) (*mix.FileOrBuffer, error) {
	logger.Logger.Info("Proxying request", "type", trackType, "segment", segmentIndex)

	if this.Options.NoTranscodeCache {
		segment, err := this.getTrackManager(trackType).GetSegment(ctx, segmentIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure raw segment exists: %w", err)
		}
		return this.transcodeSegment(ctx, segment, mix.BufferTarget())
	}

	transcodedPath, err := this.ensureSegmentExistsTranscoded(ctx, trackType, segmentIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure transcoded segment exists: %w", err)
	}
	return mix.File(transcodedPath), nil
}

func (this *RemoteHandler) ensureSegmentExistsTranscoded(ctx context.Context, trackType string, segmentIndex int) (string, error) {
	trackIndex := this.getTrackIndex(trackType)
	transcodedPath, err := this.getSegmentPath(trackType, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get transcoded segment path for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}

	// Load manifest and check if segment needs regeneration
	manifest, err := ffmpeg.LoadSegmentManifest(transcodedPath + ".json")
	needsRegeneration := err != nil || !ffmpeg.ManifestMatches(manifest, this.Options, 0)

	if _, err := os.Stat(transcodedPath); err == nil && !needsRegeneration {
		return transcodedPath, nil
	}

	segment, err := this.getTrackManager(trackType).GetSegment(ctx, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to ensure raw segment exists for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}

	_, err = this.transcodeSegment(ctx, segment, mix.FileTarget(transcodedPath))
	if err != nil {
		return "", errors.Wrapf(err, "failed to transcode segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}
	return transcodedPath, nil
}

func (this *RemoteHandler) transcodeSegment(ctx context.Context, input *mix.FileOrBuffer, target *mix.TargetFileOrBuffer) (*mix.FileOrBuffer, error) {
	var subtitle *ffmpeg.SubtitleTranscodeOptions = nil

	if this.Options.Subtitle.BurnIn {
		path := filepath.Join(this.StorageDirectory, "subtitles.vtt")
		_, err := this.getSubtitles(mix.FileTarget(path))
		if err != nil {
			return nil, fmt.Errorf("failed to get subtitles for burn-in: %w", err)
		}
		subtitle = &ffmpeg.SubtitleTranscodeOptions{
			Path:     path,
			FontSize: this.Options.Subtitle.FontSize,
		}
	}

	opts := &ffmpeg.TranscodeOptions{
		StartTime:      0,
		Duration:       0,
		Subtitle:       subtitle,
		Bitrate:        this.Options.Bitrate,
		MaxOutputWidth: this.Options.MaxOutputWidth,
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

func (this *RemoteHandler) getSubtitlePath() string {
	path := this.Options.Subtitle.Path
	if index, found := GetEmbeddedIndex(path); found {
		return filepath.Join(this.StorageDirectory, fmt.Sprintf("subtitle_%d.vtt", index))
	}
	return path
}

func (this *RemoteHandler) getSegmentPath(trackType string, segmentIndex int) (string, error) {
	trackDir, err := this.getTrackDir(trackType)
	if err != nil {
		return "", err
	}

	localPath := filepath.Join(trackDir, fmt.Sprintf("segment_%d.ts", segmentIndex))
	return localPath, nil
}

func (this *RemoteHandler) getTrackIndex(trackType string) int {
	if trackType == "video" {
		return this.Options.VideoTrack
	}
	return this.Options.AudioTrack
}

func (this *RemoteHandler) getTrackDir(trackType string) (string, error) {
	trackIndex := this.getTrackIndex(trackType)
	trackDir := filepath.Join(this.StorageDirectory, fmt.Sprintf("%s_%d", trackType, trackIndex))
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		return "", err
	}
	return trackDir, nil
}

// serveFile serves a local file
func (this *RemoteHandler) serveFile(w http.ResponseWriter, path string, contentType string) {
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
func (this *RemoteHandler) ServeSubtitles(ctx context.Context) (*mix.FileOrBuffer, error) {
	if this.Options.Subtitle.Path == "none" || this.Options.Subtitle.BurnIn {
		return nil, fmt.Errorf("no external subtitles available")
	}

	return this.getSubtitles(mix.FileTarget(filepath.Join(this.StorageDirectory, "subtitles.vtt")))
}

func (this *RemoteHandler) getSubtitles(target *mix.TargetFileOrBuffer) (*mix.FileOrBuffer, error) {
	subtitles, err := this.readSubtitles(target)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitles: %w", err)
	}
	return ProcessSubtitles(subtitles, target, this.Options.Subtitle.IgnoreClosedCaptions)
}

func (this *RemoteHandler) readSubtitles(target *mix.TargetFileOrBuffer) (*mix.FileOrBuffer, error) {
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
		subtitlePath = filepath.Join(this.StorageDirectory, fmt.Sprintf("subtitle_%d.vtt", index))
		if target.IsBuffer {
			data, err := os.ReadFile(subtitlePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read subtitle file: %w", err)
			}
			return mix.Buffer(data), nil
		} else {
			filehelper.EnsureSymlink(subtitlePath, target.FilePath)
			return target.ToOutput(), nil
		}
	}

	return nil, fmt.Errorf("unsupported subtitle path format %s", subtitlePath)
}
