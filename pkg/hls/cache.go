package hls

import (
	"io"
	"os"
	"path/filepath"
)

// CacheOperations provides low-level cache operations that can be shared
// The high-level cache management (keys, metadata) remains separate

// EnsureCacheDir creates cache directory if it doesn't exist
func EnsureCacheDir(cacheDir string) error {
	return os.MkdirAll(cacheDir, 0755)
}

// SaveToCache saves data to a cache file
func SaveToCache(cacheDir, filename string, data []byte) (string, error) {
	if err := EnsureCacheDir(cacheDir); err != nil {
		return "", err
	}

	filePath := filepath.Join(cacheDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

// SaveStreamToCache saves a stream to a cache file
func SaveStreamToCache(cacheDir, filename string, reader io.Reader) (string, error) {
	if err := EnsureCacheDir(cacheDir); err != nil {
		return "", err
	}

	filePath := filepath.Join(cacheDir, filename)
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, reader); err != nil {
		return "", err
	}

	return filePath, nil
}

// LoadFromCache loads data from a cache file
func LoadFromCache(cacheDir, filename string) ([]byte, error) {
	filePath := filepath.Join(cacheDir, filename)
	return os.ReadFile(filePath)
}

// CacheExists checks if a file exists in cache
func CacheExists(cacheDir, filename string) bool {
	filePath := filepath.Join(cacheDir, filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// GetCachePath returns the full path for a cached file
func GetCachePath(cacheDir, filename string) string {
	return filepath.Join(cacheDir, filename)
}
