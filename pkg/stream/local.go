package stream

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"wails-cast/pkg/hls"
	"wails-cast/pkg/logger"
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

	hash := md5.Sum([]byte(videoPath))
	cacheKey := hex.EncodeToString(hash[:])
	baseDir := filepath.Join(os.TempDir(), "wails-cast-hls")
	outputDir := filepath.Join(baseDir, cacheKey)
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

// ServeManifestPlaylist generates and serves the manifest HLS playlist
func (s *LocalHandler) ServeManifestPlaylist(w http.ResponseWriter, r *http.Request) {
	manifestPlaylist := &hls.ManifestPlaylist{
		Version: 3,
		VideoVariants: []hls.VideoVariant{
			{
				Index:      0,
				Bandwidth:  1500000,
				Resolution: "1280x720",
				Codecs:     "avc1.4d401f,mp4a.40.2",
				URI:        "video_0.m3u8",
			},
		},
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(manifestPlaylist.Generate()))
}

// ServeTrackPlaylist generates and serves video or audio track playlists
func (s *LocalHandler) ServeTrackPlaylist(w http.ResponseWriter, r *http.Request, trackType string, trackIndex int) {
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(trackPlaylist.Generate()))

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

	manifest, err := hls.LoadSegmentManifest(s.OutputDir, segmentIndex)
	needsRegeneration := err != nil || !hls.ManifestMatches(manifest, s.SubtitlePath, segmentDuration)

	if !hls.CacheExists(s.OutputDir, segmentName) || needsRegeneration {
		err := s.transcodeSegment(segmentPath, startTime, r, w, segmentIndex, segmentDuration)
		if err != nil {
			http.Error(w, "Transcode failed", http.StatusInternalServerError)
			logger.Logger.Error("Transcode error", "err", err)
			return
		}
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	http.ServeFile(w, r, segmentPath)
}

func (s *LocalHandler) transcodeSegment(segmentPath string, startTime float64, r *http.Request, w http.ResponseWriter, segmentIndex int, segmentDuration float64) error {
	ensureSymlink(s.VideoPath, s.OutputDir)
	opts := hls.TranscodeOptions{
		InputPath:     filepath.Join(s.OutputDir, "input_video"),
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
			return err
		}
		http.Error(w, "Transcode failed", http.StatusInternalServerError)
		return err
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
	err = hls.SaveSegmentManifest(s.OutputDir, manifest)
	return err
}

func ensureSymlink(filePath string, folder string) {
	linkPath := filepath.Join(folder, "input_video")
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		os.Symlink(filePath, linkPath)
	}
}

// Cleanup removes session files
func (s *LocalHandler) Cleanup() {
	if s.OutputDir != "" {
		os.RemoveAll(s.OutputDir)
	}
}
