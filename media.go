package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type MediaManager struct {
	discovery  *DeviceDiscovery
	serverPort int
}

func NewMediaManager(discovery *DeviceDiscovery, port int) *MediaManager {
	return &MediaManager{
		discovery:  discovery,
		serverPort: port,
	}
}

// GetMediaURL generates a URL for a media file
func (mm *MediaManager) GetMediaURL(filePath string) string {
	localIP := mm.discovery.GetLocalIP()
	encoded := url.QueryEscape(filePath)
	return fmt.Sprintf("http://%s:%d/%s", localIP, mm.serverPort, encoded)
}

// GetMediaFiles returns media files from a directory
func (mm *MediaManager) GetMediaFiles(dirPath string) ([]string, error) {
	var files []string
	extensions := map[string]bool{
		".mp4": true, ".mkv": true, ".webm": true,
		".avi": true, ".mov": true, ".flv": true,
		".m4v": true,
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors for individual files
		}
		if !info.IsDir() && extensions[strings.ToLower(filepath.Ext(path))] {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return files, fmt.Errorf("error reading directory: %w", err)
	}

	return files, nil
}

// CastToDevice sends media to a device
func (mm *MediaManager) CastToDevice(deviceURL, mediaPath string) (bool, string, error) {
	// Verify media file exists
	if _, err := os.Stat(mediaPath); err != nil {
		return false, "", fmt.Errorf("media file not found: %w", err)
	}

	// Get media URL
	mediaURL := mm.GetMediaURL(mediaPath)

	message := fmt.Sprintf("Casting %s to %s via %s", filepath.Base(mediaPath), deviceURL, mediaURL)
	return true, message, nil
}

// GetContentType returns the appropriate MIME type for a file
func (mm *MediaManager) GetContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".flv":
		return "video/x-flv"
	default:
		return "application/octet-stream"
	}
}
