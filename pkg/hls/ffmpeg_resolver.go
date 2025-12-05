package hls

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ffmpegPath  string
	ffprobePath string
)

// FFmpegInfo contains information about ffmpeg and ffprobe installation
type FFmpegInfo struct {
	FFmpegInstalled  bool   `json:"ffmpegInstalled"`
	FFprobeInstalled bool   `json:"ffprobeInstalled"`
	FFmpegVersion    string `json:"ffmpegVersion"`
	FFprobeVersion   string `json:"ffprobeVersion"`
	FFmpegPath       string `json:"ffmpegPath"`
	FFprobePath      string `json:"ffprobePath"`
}

// findExecutable locates an executable in common system paths
func findExecutable(name string) string {
	// Common installation paths for macOS
	commonPaths := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
		"/usr/bin",
		"/opt/local/bin",
	}

	// First try exec.LookPath with current PATH
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	// Then check common installation directories
	for _, dir := range commonPaths {
		fullPath := filepath.Join(dir, name)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	// Fall back to just the name (will fail if not in PATH)
	return name
}

// initPaths initializes the ffmpeg and ffprobe paths
func initPaths(forceRefresh bool) {
	if forceRefresh || ffmpegPath == "" || ffprobePath == "" {
		ffmpegPath = findExecutable("ffmpeg")
		ffprobePath = findExecutable("ffprobe")
	}
}

// GetFFmpegInfo returns version and path information for ffmpeg and ffprobe
func GetFFmpegInfo(searchAgain bool) (*FFmpegInfo, error) {
	initPaths(searchAgain)

	info := &FFmpegInfo{
		FFmpegPath:  ffmpegPath,
		FFprobePath: ffprobePath,
	}

	// Check ffmpeg
	if _, err := os.Stat(ffmpegPath); err == nil {
		info.FFmpegInstalled = true
		if version := getVersion(ffmpegPath); version != "" {
			info.FFmpegVersion = version
		}
	}

	// Check ffprobe
	if _, err := os.Stat(ffprobePath); err == nil {
		info.FFprobeInstalled = true
		if version := getVersion(ffprobePath); version != "" {
			info.FFprobeVersion = version
		}
	}

	// Return error if either is missing
	if !info.FFmpegInstalled || !info.FFprobeInstalled {
		var missing []string
		if !info.FFmpegInstalled {
			missing = append(missing, "ffmpeg")
		}
		if !info.FFprobeInstalled {
			missing = append(missing, "ffprobe")
		}
		return info, fmt.Errorf("%s not found in PATH or common installation directories. Please install ffmpeg (e.g., 'brew install ffmpeg')", strings.Join(missing, " and "))
	}

	return info, nil
}

// getVersion extracts version from ffmpeg/ffprobe
func getVersion(execPath string) string {
	cmd := exec.Command(execPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse first line to extract version
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		// Extract version number from line like "ffmpeg version 6.0"
		parts := strings.Fields(lines[0])
		if len(parts) >= 3 {
			return parts[2]
		}
	}

	return ""
}
