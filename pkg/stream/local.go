package stream

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wails-cast/pkg/hls"
)

// LocalHLSServer represents a local file HLS streaming server
type LocalHLSServer struct {
	VideoPath    string
	SubtitlePath string
	OutputDir    string
	Duration     float64
	SegmentSize  int
	LocalIP      string
}

// NewLocalHLSServer creates a new local HLS server
func NewLocalHLSServer(videoPath, subtitlePath, localIP string) *LocalHLSServer {
	duration, err := hls.GetVideoDuration(videoPath)
	if err != nil {
		duration = 0
	}

	sessionID := filepath.Base(videoPath)
	baseDir := filepath.Join(os.TempDir(), "wails-cast-hls")
	outputDir := filepath.Join(baseDir, sessionID)
	hls.EnsureCacheDir(outputDir)

	return &LocalHLSServer{
		VideoPath:    videoPath,
		SubtitlePath: subtitlePath,
		OutputDir:    outputDir,
		Duration:     duration,
		SegmentSize:  8,
		LocalIP:      localIP,
	}
}

// ServePlaylist generates and serves the HLS playlist
func (s *LocalHLSServer) ServePlaylist(w http.ResponseWriter, r *http.Request) {
	playlistContent := hls.GenerateVODPlaylist(s.Duration, s.SegmentSize, s.LocalIP, 8888)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")

	w.Write([]byte(playlistContent))
}

// ServeSegment transcodes and serves a segment
func (s *LocalHLSServer) ServeSegment(w http.ResponseWriter, r *http.Request, segmentName string) {
	hls.EnsureCacheDir(s.OutputDir)

	segmentNum, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(segmentName, "segment"), ".ts"))
	if err != nil {
		http.Error(w, "Invalid segment name", http.StatusBadRequest)
		return
	}

	segmentPath := hls.GetCachePath(s.OutputDir, segmentName)
	segmentDuration := float64(s.SegmentSize)
	startTime := float64(segmentNum * s.SegmentSize)
	if startTime+segmentDuration > s.Duration {
		segmentDuration = s.Duration - startTime
	}

	needsRegeneration := false
	manifest, err := hls.LoadSegmentManifest(s.OutputDir, segmentNum)
	if err != nil || !hls.ManifestMatches(manifest, s.SubtitlePath, segmentDuration) {
		needsRegeneration = true
	}

	if !hls.CacheExists(s.OutputDir, segmentName) || needsRegeneration {
		opts := hls.TranscodeOptions{
			InputPath:    s.VideoPath,
			OutputPath:   segmentPath,
			StartTime:    startTime,
			Duration:     s.SegmentSize,
			SubtitlePath: s.SubtitlePath,
			Preset:       "veryfast",
		}

		result := hls.TranscodeSegment(r.Context(), opts, true)
		if result.Error != nil {
			if r.Context().Err() != nil {
				return
			}
			http.Error(w, "Transcode failed", http.StatusInternalServerError)
			return
		}

		manifest := hls.SegmentManifest{
			SegmentNumber: segmentNum,
			Duration:      segmentDuration,
			SubtitlePath:  s.SubtitlePath,
			SubtitleStyle: "FontSize=24",
			VideoCodec:    "libx264",
			AudioCodec:    "aac",
			Preset:        "veryfast",
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
func (s *LocalHLSServer) Cleanup() {
	if s.OutputDir != "" {
		os.RemoveAll(s.OutputDir)
	}
}
