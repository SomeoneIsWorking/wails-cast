package stream

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"wails-cast/pkg/hls"
)

// LocalHandler represents a local file HLS streaming server
type LocalHandler struct {
	VideoPath    string
	SubtitlePath string
	Options      StreamOptions
	OutputDir    string
	Duration     float64
	SegmentSize  int
	LocalIP      string
}

// NewLocalHandler creates a new local HLS handler
func NewLocalHandler(videoPath string, options StreamOptions, localIP string) *LocalHandler {
	duration, err := hls.GetVideoDuration(videoPath)
	if err != nil {
		duration = 0
	}

	sessionID := filepath.Base(videoPath)
	baseDir := filepath.Join(os.TempDir(), "wails-cast-hls")
	outputDir := filepath.Join(baseDir, sessionID)
	hls.EnsureCacheDir(outputDir)

	return &LocalHandler{
		VideoPath:    videoPath,
		SubtitlePath: options.SubtitlePath,
		Options:      options,
		OutputDir:    outputDir,
		Duration:     duration,
		SegmentSize:  8,
		LocalIP:      localIP,
	}
}

// ServeMainPlaylist generates and serves the master HLS playlist
func (s *LocalHandler) ServeMainPlaylist(w http.ResponseWriter, r *http.Request) {
	// Generate master playlist pointing to /video_0.m3u8
	// Since local files usually have 1 video track, we just point to it.
	// If we had multiple qualities, we would list them here.

	masterPlaylist := "#EXTM3U\n#EXT-X-VERSION:3\n"

	// Add video track
	// We can add bandwidth info if we knew it, or just default.
	masterPlaylist += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080\n%s/video_0.m3u8\n", s.LocalIP)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(masterPlaylist))
}

// ServeTrackPlaylist generates and serves video or audio track playlists
func (s *LocalHandler) ServeTrackPlaylist(w http.ResponseWriter, r *http.Request, trackType string, trackIndex int) {
	if trackType == "video" && trackIndex == 0 {
		// For local files, we only have video_0
		playlistContent := hls.GenerateVODPlaylist(s.Duration, s.SegmentSize, s.LocalIP, 8888)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte(playlistContent))
		return
	}

	http.NotFound(w, r)
}

// ServeSegment transcodes and serves a segment
func (s *LocalHandler) ServeSegment(w http.ResponseWriter, r *http.Request, trackType string, trackIndex int, segmentIndex int) {
	segmentName := filepath.Base(r.URL.Path)

	hls.EnsureCacheDir(s.OutputDir)

	segmentPath := hls.GetCachePath(s.OutputDir, segmentName)
	segmentDuration := float64(s.SegmentSize)
	startTime := float64(segmentIndex * s.SegmentSize)
	if startTime+segmentDuration > s.Duration {
		segmentDuration = s.Duration - startTime
	}

	needsRegeneration := false
	manifest, err := hls.LoadSegmentManifest(s.OutputDir, segmentIndex)
	if err != nil || !hls.ManifestMatches(manifest, s.SubtitlePath, segmentDuration) {
		needsRegeneration = true
	}

	if !hls.CacheExists(s.OutputDir, segmentName) || needsRegeneration {
		opts := hls.TranscodeOptions{
			InputPath:     s.VideoPath,
			OutputPath:    segmentPath,
			StartTime:     startTime,
			Duration:      s.SegmentSize,
			SubtitlePath:  s.Options.SubtitlePath,
			SubtitleTrack: s.Options.SubtitleTrack,
			BurnIn:        s.Options.BurnIn,
			Quality:       s.Options.Quality,
		}

		err := hls.TranscodeSegment(r.Context(), opts)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			http.Error(w, "Transcode failed", http.StatusInternalServerError)
			return
		}

		manifest := hls.SegmentManifest{
			SegmentNumber: segmentIndex,
			Duration:      segmentDuration,
			SubtitlePath:  s.Options.SubtitlePath,
			SubtitleStyle: "FontSize=24",
			VideoCodec:    "libx264",
			AudioCodec:    "aac",
			Preset:        "fast",
			CreatedAt:     time.Now().Format(time.RFC3339),
		}
		hls.SaveSegmentManifest(s.OutputDir, manifest)
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	http.ServeFile(w, r, segmentPath)
}

// Cleanup removes session files
func (s *LocalHandler) Cleanup() {
	if s.OutputDir != "" {
		os.RemoveAll(s.OutputDir)
	}
}
