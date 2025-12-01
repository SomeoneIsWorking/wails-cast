package hls

import (
	"encoding/json"
	"os"

	"wails-cast/pkg/options"
)

// SegmentManifest stores metadata for each segment
// This is used by BOTH local file HLS and remote proxy HLS
type SegmentManifest struct {
	Duration  float64 `json:"duration"`
	Subtitle  string  `json:"subtitle"`
	CreatedAt string  `json:"created_at"`
	Bitrate   string  `json:"bitrate"`
}

// Save saves manifest JSON for a segment (used by local file HLS)
func (this *SegmentManifest) Save(manifestPath string) error {
	data, err := json.MarshalIndent(this, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

// LoadSegmentManifest loads manifest JSON for a segment (used by local file HLS)
func LoadSegmentManifest(manifestPath string) (*SegmentManifest, error) {
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
func ManifestMatches(manifest *SegmentManifest, options options.CastOptions, duration float64) bool {
	if manifest == nil {
		return false
	}

	if options.Subtitle.BurnIn {
		if manifest.Subtitle != options.Subtitle.Path {
			return false
		}
	}

	// Check if duration changed significantly (tolerance of 0.1s)
	if manifest.Duration > 0 && (manifest.Duration-duration > 0.1 || duration-manifest.Duration > 0.1) {
		return false
	}

	if manifest.Bitrate != options.Bitrate {
		return false
	}

	return true
}
