package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"wails-cast/pkg/subtitles"
)

// GeneratePromptFromSubtitles builds the prompt text from exported subtitle files
func GeneratePromptFromSubtitles(exportedDir string, targetLanguage string, promptTemplate string, maxSamples int) (string, error) {
	entries, err := os.ReadDir(exportedDir)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle directory: %w", err)
	}

	type subtitleFile struct {
		path     string
		filename string
		priority int
	}

	var allSubtitles []subtitleFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".vtt") {
			continue
		}
		subtitlePath := filepath.Join(exportedDir, entry.Name())
		allSubtitles = append(allSubtitles, subtitleFile{path: subtitlePath, filename: entry.Name(), priority: scoreLangPriority(entry.Name())})
	}

	if len(allSubtitles) == 0 {
		return "", fmt.Errorf("no subtitle files found in directory")
	}

	// sort by priority simple bubble (small dataset)
	for i := 0; i < len(allSubtitles); i++ {
		for j := i + 1; j < len(allSubtitles); j++ {
			if allSubtitles[j].priority > allSubtitles[i].priority {
				allSubtitles[i], allSubtitles[j] = allSubtitles[j], allSubtitles[i]
			}
		}
	}

	if len(allSubtitles) > maxSamples {
		allSubtitles = allSubtitles[:maxSamples]
	}

	var subtitleContents []string
	for _, sub := range allSubtitles {
		content, err := os.ReadFile(sub.path)
		if err != nil {
			return "", fmt.Errorf("failed to read subtitle file %s: %w", sub.filename, err)
		}
		vttJson, err := subtitles.Parse(string(content))
		if err != nil {
			return "", fmt.Errorf("failed to parse subtitle file %s: %w", sub.filename, err)
		}
		simpleFormat := vttJson.ToSimpleFormat()
		filename := strings.TrimSuffix(sub.filename, ".vtt")
		subtitleContents = append(subtitleContents, fmt.Sprintf("<input_subtitle_%s>\n%s</input_subtitle_%s>", filename, simpleFormat, filename))
	}

	combined := strings.Join(subtitleContents, "\n\n")

	tmpl, err := template.New("translate").Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, map[string]string{"TargetLanguage": targetLanguage, "SubtitleContent": combined}); err != nil {
		return "", fmt.Errorf("failed to execute prompt template: %w", err)
	}

	return sb.String(), nil
}

// ProcessPastedAnswer accepts an LLM output (or raw text), extracts the useful portion and writes a WebVTT file
func ProcessPastedAnswer(ctx context.Context, pasted string, exportedDir string, targetLanguage string) ([]string, error) {
	if pasted == "" {
		return nil, fmt.Errorf("no pasted content provided")
	}

	// Extract between <llm_output> tags if present
	startTag := "<llm_output>"
	endTag := "</llm_output>"
	content := pasted
	startIdx := strings.Index(pasted, startTag)
	endIdx := strings.Index(pasted, endTag)
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		content = pasted[startIdx+len(startTag) : endIdx]
	}
	content = strings.TrimSpace(content)

	// Parse simple format into internal representation
	vttJson, err := subtitles.ParseSimpleFormat(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pasted content: %w", err)
	}

	vttContent := vttJson.ToWebVTTString()

	// Ensure output dir exists
	if err := os.MkdirAll(exportedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create subtitle directory: %w", err)
	}

	outputPath := filepath.Join(exportedDir, fmt.Sprintf("%s.vtt", targetLanguage))
	if err := os.WriteFile(outputPath, []byte(vttContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write translated subtitle: %w", err)
	}

	return []string{outputPath}, nil
}
