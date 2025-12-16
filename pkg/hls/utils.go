package hls

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"wails-cast/pkg/logger"
)

func ExportEmbeddedSubtitles(videoPath string) error {
	if strings.HasPrefix(videoPath, "http://") || strings.HasPrefix(videoPath, "https://") {
		return fmt.Errorf("cannot export subtitles from remote URLs")
	}

	trackInfo, err := GetMediaTrackInfo(videoPath)
	if err != nil {
		return fmt.Errorf("failed to get track info: %w", err)
	}

	if len(trackInfo.SubtitleTracks) == 0 {
		return fmt.Errorf("no embedded subtitles found")
	}

	baseDir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	subtitleDir := filepath.Join(baseDir, baseName)

	// Create subtitle directory
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return fmt.Errorf("failed to create subtitle directory: %w", err)
	}

	for _, sub := range trackInfo.SubtitleTracks {
		name := sub.Title
		if name == "" {
			name = sub.Language
		}
		if name == "" {
			name = fmt.Sprintf("track%d", sub.Index)
		}

		outputFile := filepath.Join(subtitleDir, fmt.Sprintf("%s.vtt", name))

		// Use ffmpeg to extract subtitle
		args := []string{
			"-i", videoPath,
			"-map", fmt.Sprintf("0:s:%d", sub.Index),
			"-f", "webvtt",
			"-y", // Overwrite output file
			outputFile,
		}

		if err := RunFFmpeg(args...); err != nil {
			logger.Logger.Warn("Failed to export subtitle", "index", sub.Index, "language", name, "error", err)
			continue
		}

		logger.Logger.Info("Exported subtitle", "file", outputFile)
	}

	return nil
}

// Define characters that are illegal or problematic on most major operating systems (Windows, Unix).
// This includes control characters and filesystem-specific separators/wildcards.
// The regex pattern matches: < > : " / \ | ? *
var illegalChars = regexp.MustCompile(`[<>:"/\\|?*]`)

// reservedNames are Windows system names that can cause issues if used as a filename (without extension).
// Pre-calculating the map improves performance over using a switch or slice search.
var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// ConvertToUsableFilename cleans an arbitrary string to create a safe filename.
func ConvertToUsableFilename(s string) string {
	// 1. Trim leading/trailing whitespace
	cleaned := strings.TrimSpace(s)

	// 2. Replace illegal characters with a safe substitute (underscore)
	cleaned = illegalChars.ReplaceAllString(cleaned, "_")

	// 3. Handle edge case of an empty string after cleaning
	if cleaned == "" {
		return "default_file"
	}

	// 4. Truncate long filenames to prevent OS path limit issues (e.g., max 255 bytes)
	const maxLength = 200
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}

	// 5. Check for and neutralize Windows reserved filenames (case-insensitive)
	baseName := cleaned

	// Separate base name from extension, if present
	if dotIndex := strings.LastIndex(cleaned, "."); dotIndex != -1 {
		baseName = cleaned[:dotIndex]
	}

	// If the base name (uppercase) matches a reserved name, prepend an underscore
	if reservedNames[strings.ToUpper(baseName)] {
		// Only prepend if it's an exact match of a reserved word, not just a file that STARTS with it.
		cleaned = "_" + cleaned
	}

	return cleaned
}
