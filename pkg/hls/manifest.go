package hls

import (
	"encoding/json"
	"os"
	"wails-cast/pkg/options"
)

// Save saves manifest JSON for a segment (used by local file HLS)
func (this *TranscodeOptions) Save(manifestPath string) error {
	data, err := json.MarshalIndent(this, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

// LoadSegmentManifest loads manifest JSON for a segment (used by local file HLS)
func LoadSegmentManifest(manifestPath string) (*TranscodeOptions, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest TranscodeOptions
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ManifestMatches checks if current parameters match manifest (used by local file HLS)
func ManifestMatches(manifest *TranscodeOptions, options options.CastOptions, duration int) bool {
	if manifest == nil {
		return false
	}

	if options.Subtitle.BurnIn {
		if manifest.Subtitle != options.Subtitle.Path {
			return false
		}

		if manifest.FontSize != options.Subtitle.FontSize {
			return false
		}
	}

	if !options.Subtitle.BurnIn && manifest.Subtitle != "" {
		return false
	}

	if manifest.MaxOutputWidth != options.MaxOutputWidth {
		return false
	}

	if manifest.Duration != duration {
		return false
	}

	if manifest.Bitrate != options.Bitrate {
		return false
	}

	return true
}
