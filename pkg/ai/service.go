package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	localcast "wails-cast/pkg/cast"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
)

// Request bundles parameters for prompt generation, pasted processing and translation
type Request struct {
	FileNameOrURL  string
	TargetLanguage string
	APIKey         string
	Model          string
	PromptTemplate string
	MaxSamples     int
}

// resolveSubtitleDir determines the directory where exported subtitles live; it can export embedded subtitles if needed
func resolveSubtitleDir(fileNameOrUrl string, exportIfMissing bool) (string, error) {
	isRemote := strings.HasPrefix(fileNameOrUrl, "http://") || strings.HasPrefix(fileNameOrUrl, "https://")
	if isRemote {
		if _, err := localcast.GetRemoteTrackInfo(fileNameOrUrl); err != nil {
			return "", fmt.Errorf("failed to extract track info: %w", err)
		}
		return folders.Video(fileNameOrUrl), nil
	}

	baseDir := filepath.Dir(fileNameOrUrl)
	baseName := strings.TrimSuffix(filepath.Base(fileNameOrUrl), filepath.Ext(fileNameOrUrl))
	subtitleDir := filepath.Join(baseDir, baseName)

	if _, err := os.Stat(subtitleDir); os.IsNotExist(err) {
		if exportIfMissing {
			if err := hls.ExportEmbeddedSubtitles(fileNameOrUrl); err != nil {
				return "", fmt.Errorf("failed to export subtitles: %w", err)
			}
		} else {
			return "", fmt.Errorf("subtitle directory does not exist")
		}
	}

	return subtitleDir, nil
}

// GeneratePromptForFile resolves subtitle directory and generates the prompt
func GeneratePromptForFile(req Request) (string, error) {
	subtitleDir, err := resolveSubtitleDir(req.FileNameOrURL, true)
	if err != nil {
		return "", err
	}
	return GeneratePromptFromSubtitles(subtitleDir, req.TargetLanguage, req.PromptTemplate, req.MaxSamples)
}

// ProcessPastedForFile resolves subtitle directory and processes pasted LLM output
func ProcessPastedForFile(ctx context.Context, req Request, pastedAnswer string) ([]string, error) {
	subtitleDir, err := resolveSubtitleDir(req.FileNameOrURL, true)
	if err != nil {
		return nil, err
	}
	return ProcessPastedAnswer(ctx, pastedAnswer, subtitleDir, req.TargetLanguage)
}

// TranslateForFile runs the full translation synchronously and returns the output paths
func TranslateForFile(ctx context.Context, req Request) ([]string, error) {
	subtitleDir, err := resolveSubtitleDir(req.FileNameOrURL, true)
	if err != nil {
		return nil, err
	}

	translator, err := NewTranslator(req.APIKey, req.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create translator: %w", err)
	}
	defer translator.Close()

	return translator.TranslateEmbeddedSubtitles(ctx, TranslateOptions{
		ExportedSubtitlesDir: subtitleDir,
		TargetLanguage:       req.TargetLanguage,
		PromptTemplate:       req.PromptTemplate,
		MaxSubtitleSamples:   req.MaxSamples,
	})
}
