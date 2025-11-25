package hlsproxy

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// SegmentCombiner handles combining demuxed audio and video segments
type SegmentCombiner struct {
	audioTrackURL string
	videoTrackURL string
	cacheDir      string
	mu            sync.Mutex
}

// NewSegmentCombiner creates a new segment combiner
func NewSegmentCombiner(audioTrackURL, videoTrackURL, cacheDir string) *SegmentCombiner {
	return &SegmentCombiner{
		audioTrackURL: audioTrackURL,
		videoTrackURL: videoTrackURL,
		cacheDir:      cacheDir,
	}
}

// GetSegmentPair determines which audio and video segments correspond to each other
// For demuxed HLS, segments are typically numbered the same (e.g., segment1.ts in both playlists)
func (sc *SegmentCombiner) GetSegmentPair(segmentURL string) (audioURL, videoURL string) {
	// Extract the segment filename/pattern
	lastSlash := strings.LastIndexAny(segmentURL, "/")
	if lastSlash == -1 {
		return "", ""
	}

	segmentName := segmentURL[lastSlash+1:]

	// Get base URL for audio and video tracks
	audioBase := sc.audioTrackURL[:strings.LastIndex(sc.audioTrackURL, "/")+1]
	videoBase := sc.videoTrackURL[:strings.LastIndex(sc.videoTrackURL, "/")+1]

	// Construct full URLs
	audioURL = audioBase + segmentName
	videoURL = videoBase + segmentName

	return audioURL, videoURL
}

// CombineSegments combines an audio segment and video segment into a single muxed MPEG-TS file
func (sc *SegmentCombiner) CombineSegments(audioPath, videoPath, outputPath string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Use ffmpeg to combine audio and video into a single MPEG-TS file
	// This is much better than serving separate tracks to Chromecast
	cmd := exec.Command("ffmpeg",
		"-i", videoPath, // Video input
		"-i", audioPath, // Audio input
		"-c:v", "copy", // Copy video stream (already correct format)
		"-c:a", "aac", // Re-encode audio to AAC
		"-b:a", "128k",
		"-ar", "48000",
		"-ac", "2",
		"-f", "mpegts", // Output format: MPEG-TS
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg combine failed: %v\nOutput: %s", err, string(output))
	}

	return nil
}

// ParseSegmentNumber extracts the segment number from a URL
// Assumes format like "segment10.ts" or "i10.jpg"
func ParseSegmentNumber(url string) int {
	lastSlash := strings.LastIndexAny(url, "/")
	if lastSlash != -1 {
		url = url[lastSlash+1:]
	}

	// Remove extension
	if idx := strings.LastIndex(url, "."); idx != -1 {
		url = url[:idx]
	}

	// Extract number
	var num int
	// Try different patterns
	if _, err := fmt.Sscanf(url, "segment%d", &num); err == nil {
		return num
	}
	if _, err := fmt.Sscanf(url, "i%d", &num); err == nil {
		return num
	}

	// Fallback: scan for any number
	fmt.Sscanf(url, "%d", &num)
	return num
}
