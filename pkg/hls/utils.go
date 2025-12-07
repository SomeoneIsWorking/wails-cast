package hls

import (
	"fmt"
	"os"
	"path/filepath"
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
		lang := sub.Language
		if lang == "" {
			lang = fmt.Sprintf("track%d", sub.Index)
		}

		outputFile := filepath.Join(subtitleDir, fmt.Sprintf("%s.vtt", lang))

		// Use ffmpeg to extract subtitle
		args := []string{
			"-i", videoPath,
			"-map", fmt.Sprintf("0:s:%d", sub.Index),
			"-f", "webvtt",
			"-y", // Overwrite output file
			outputFile,
		}

		if err := RunFFmpeg(args...); err != nil {
			logger.Logger.Warn("Failed to export subtitle", "index", sub.Index, "language", lang, "error", err)
			continue
		}

		logger.Logger.Info("Exported subtitle", "file", outputFile)
	}

	return nil
}
