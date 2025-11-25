package util

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseDeviceURL extracts host and port from a device URL
func ParseDeviceURL(deviceURL string) (string, int, error) {
	deviceAddr := deviceURL

	// Strip protocol if present
	if strings.HasPrefix(deviceAddr, "http://") {
		deviceAddr = strings.TrimPrefix(deviceAddr, "http://")
	} else if strings.HasPrefix(deviceAddr, "https://") {
		deviceAddr = strings.TrimPrefix(deviceAddr, "https://")
	}

	host := deviceAddr
	port := 8009

	// Check if port is already included
	if strings.Contains(deviceAddr, ":") {
		parts := strings.SplitN(deviceAddr, ":", 2)
		host = parts[0]
		if len(parts) == 2 {
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
			}
		}
	}

	return host, port, nil
}

// ExtractFileName extracts a filename or description from a URL
func ExtractFileName(url string) string {
	// Try to get the last path segment
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		// Remove query parameters
		if idx := strings.Index(lastPart, "?"); idx != -1 {
			lastPart = lastPart[:idx]
		}
		if lastPart != "" {
			return lastPart
		}
	}
	return filepath.Base(url)
}

// ExtractDeviceName extracts device name from a device URL
func ExtractDeviceName(deviceURL string) string {
	// Extract name from URL or return address
	parts := strings.Split(deviceURL, "/")
	if len(parts) > 2 {
		return parts[2]
	}
	return deviceURL
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
