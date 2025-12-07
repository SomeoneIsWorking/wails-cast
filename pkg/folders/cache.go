package folders

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CacheStats holds cache size information
type CacheStats struct {
	TotalSize       int64 `json:"totalSize"`
	TranscodedSize  int64 `json:"transcodedSize"`
	RawSegmentsSize int64 `json:"rawSegmentsSize"`
	MetadataSize    int64 `json:"metadataSize"`
}

// GetCacheStats calculates cache statistics
func GetCacheStats() (*CacheStats, error) {
	cachePath := GetCache()
	stats := &CacheStats{}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return stats, nil
	}

	err := filepath.WalkDir(cachePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		size := info.Size()
		stats.TotalSize += size

		ext := strings.ToLower(filepath.Ext(path))

		switch {
		case ext == ".json":
			stats.MetadataSize += size
		case ext == ".ts":
			// Check if it's a raw segment or transcoded segment
			if strings.HasSuffix(path, "_raw.ts") {
				stats.RawSegmentsSize += size
			} else {
				// Transcoded segments (may or may not have .json manifest)
				stats.TranscodedSize += size
			}
		default:
			// Other files (m3u8, etc) count as metadata
			stats.MetadataSize += size
		}

		return nil
	})

	return stats, err
}

// DeleteAllCache removes all cache files
func DeleteAllCache() error {
	cachePath := GetCache()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(cachePath)
}

// DeleteTranscodedCache removes only transcoded video segments
func DeleteTranscodedCache() error {
	cachePath := GetCache()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(cachePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Only delete .ts files that have a corresponding .json manifest
		if strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, "_raw.ts") {
			jsonManifest := path + ".json"
			// This is a transcoded segment, delete both the .ts and .json files
			os.Remove(path)
			os.Remove(jsonManifest) // Ignore error if it doesn't exist
		}

		return nil
	})
}

// DeleteAllVideoCache removes all video files (.ts) but keeps metadata
func DeleteAllVideoCache() error {
	cachePath := GetCache()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(cachePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Delete all .ts files and their .json manifests
		if strings.HasSuffix(path, ".ts") {
			os.Remove(path)
			// Also remove corresponding .json manifest if it exists
			jsonManifest := path + ".json"
			os.Remove(jsonManifest) // Ignore error if it doesn't exist
		}

		return nil
	})
}
