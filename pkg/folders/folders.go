package folders

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	appName = "wails-cast"
)

// GetConfig returns the application config directory path
func GetConfig() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	return filepath.Join(configDir, appName)
}

// GetCache returns the application cache directory path
func GetCache() string {
	return filepath.Join(os.TempDir(), appName+"-cache")
}

func GetCacheForVideo(fileNameOrUrl string) string {
	hash := md5.Sum([]byte(fileNameOrUrl))
	cacheKey := hex.EncodeToString(hash[:])
	return filepath.Join(GetCache(), cacheKey)
}

func GetTrackDir(fileNameOrUrl string, mediaType string, track int) string {
	cacheDir := GetCacheForVideo(fileNameOrUrl)
	trackDir := filepath.Join(cacheDir, fmt.Sprintf("%s_%d", mediaType, track))
	return trackDir
}
