package hls

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"wails-cast/pkg/execresolver"
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

// initPaths initializes the ffmpeg and ffprobe paths
func initPaths(forceRefresh bool) {
	if forceRefresh {
		ffmpegPath = execresolver.FindRefresh("ffmpeg")
		ffprobePath = execresolver.FindRefresh("ffprobe")
	} else if ffmpegPath == "" || ffprobePath == "" {
		ffmpegPath = execresolver.Find("ffmpeg")
		ffprobePath = execresolver.Find("ffprobe")
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
		if version := getFFmpegVersion(ffmpegPath); version != "" {
			info.FFmpegVersion = version
		}
	}

	// Check ffprobe
	if _, err := os.Stat(ffprobePath); err == nil {
		info.FFprobeInstalled = true
		if version := getFFmpegVersion(ffprobePath); version != "" {
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
		return info, fmt.Errorf("%s not found in PATH or common installation directories", joinWithAnd(missing))
	}

	return info, nil
}

// getFFmpegVersion extracts version from ffmpeg/ffprobe
func getFFmpegVersion(execPath string) string {
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

func joinWithAnd(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) == 2 {
		return items[0] + " and " + items[1]
	}
	return ""
}
