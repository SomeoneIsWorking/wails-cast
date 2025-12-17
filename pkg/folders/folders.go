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

// Cache returns the application cache directory path
func Cache() string {
	return filepath.Join(os.TempDir(), appName+"-cache")
}

func Video(fileNameOrUrl string) string {
	hash := md5.Sum([]byte(fileNameOrUrl))
	cacheKey := hex.EncodeToString(hash[:])
	return filepath.Join(Cache(), cacheKey)
}

func Track(fileNameOrUrl string, mediaType string, track int) string {
	cacheDir := Video(fileNameOrUrl)
	trackDir := filepath.Join(cacheDir, fmt.Sprintf("%s_%d", mediaType, track))
	return trackDir
}
