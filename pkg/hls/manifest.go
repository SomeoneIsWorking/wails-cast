package hls

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SegmentManifest stores metadata for each segment
// This is used by BOTH local file HLS and remote proxy HLS
type SegmentManifest struct {
	SegmentNumber int     `json:"segment_number"`
	Duration      float64 `json:"duration"`
	SubtitlePath  string  `json:"subtitle_path"`
	SubtitleStyle string  `json:"subtitle_style"`
	VideoCodec    string  `json:"video_codec"`
	AudioCodec    string  `json:"audio_codec"`
	Preset        string  `json:"preset"`
	CreatedAt     string  `json:"created_at"`
}

// SaveSegmentManifest saves manifest JSON for a segment (used by local file HLS)
func SaveSegmentManifest(outputDir string, manifest SegmentManifest) error {
	manifestPath := filepath.Join(outputDir, fmt.Sprintf("segment%d.json", manifest.SegmentNumber))
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

// LoadSegmentManifest loads manifest JSON for a segment (used by local file HLS)
func LoadSegmentManifest(outputDir string, segmentNum int) (*SegmentManifest, error) {
	manifestPath := filepath.Join(outputDir, fmt.Sprintf("segment%d.json", segmentNum))
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest SegmentManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ManifestMatches checks if current parameters match manifest (used by local file HLS)
func ManifestMatches(manifest *SegmentManifest, subtitlePath string, duration float64) bool {
	if manifest == nil {
		return false
	}
	// Check subtitle path
	if manifest.SubtitlePath != subtitlePath {
		return false
	}
	// Check if duration changed significantly (tolerance of 0.1s)
	if manifest.Duration > 0 && (manifest.Duration-duration > 0.1 || duration-manifest.Duration > 0.1) {
		return false
	}
	// Check subtitle style (currently hardcoded FontSize=24)
	expectedStyle := "FontSize=24"
	if manifest.SubtitleStyle != expectedStyle {
		return false
	}
	return true
}
