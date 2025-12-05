package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wails-cast/pkg/subtitles"

	"google.golang.org/genai"
)

const (
	defaultModel       = "gemini-2.5-flash"
	maxSubtitleSamples = 4
)

// Priority languages for translation reference
var priorityLanguages = []string{"eng", "jpn", "fre", "fra", "ita", "spa", "ger", "deu"}

// scoreLangPriority returns a score for language priority (higher is better)
func scoreLangPriority(filename string) int {
	lower := strings.ToLower(filename)
	for i, lang := range priorityLanguages {
		if strings.Contains(lower, lang) {
			return len(priorityLanguages) - i
		}
	}
	return 0
}

// Translator handles AI-powered subtitle translation
type Translator struct {
	client *genai.Client
	model  string
}

// NewTranslator creates a new translator instance
func NewTranslator(apiKey string, model string) (*Translator, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Use provided model or fallback to default
	if model == "" {
		model = defaultModel
	}

	return &Translator{
		client: client,
		model:  model,
	}, nil
}

// Close closes the client connection
func (t *Translator) Close() error {
	// Note: genai.Client doesn't have a Close method in current version
	// Keep this for future compatibility
	return nil
}

// TranslateEmbeddedSubtitles exports and translates all embedded subtitles
func (t *Translator) TranslateEmbeddedSubtitles(ctx context.Context, exportedSubtitlesDir, targetLanguage string, streamCallback func(chunk string)) ([]string, error) {
	var translatedFiles []string

	// Read all .vtt files in the directory
	entries, err := os.ReadDir(exportedSubtitlesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitle directory: %w", err)
	}

	// Collect all subtitle files with priority scoring
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

		subtitlePath := filepath.Join(exportedSubtitlesDir, entry.Name())
		allSubtitles = append(allSubtitles, subtitleFile{
			path:     subtitlePath,
			filename: entry.Name(),
			priority: scoreLangPriority(entry.Name()),
		})
	}

	if len(allSubtitles) == 0 {
		return nil, fmt.Errorf("no subtitle files found in directory")
	}

	// Sort by priority (highest first), then alphabetically
	for i := 0; i < len(allSubtitles); i++ {
		for j := i + 1; j < len(allSubtitles); j++ {
			if allSubtitles[j].priority > allSubtitles[i].priority {
				allSubtitles[i], allSubtitles[j] = allSubtitles[j], allSubtitles[i]
			}
		}
	}

	// Take only the top N subtitles
	if len(allSubtitles) > maxSubtitleSamples {
		allSubtitles = allSubtitles[:maxSubtitleSamples]
	}

	// Collect subtitle content
	var subtitleContents []string
	var subtitlePaths []string

	for _, sub := range allSubtitles {
		content, err := os.ReadFile(sub.path)
		if err != nil {
			return nil, fmt.Errorf("failed to read subtitle file %s: %w", sub.filename, err)
		}

		subtitlePaths = append(subtitlePaths, sub.path)

		// Parse WebVTT to JSON format
		vttJson, err := subtitles.Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse subtitle file %s: %w", sub.filename, err)
		}

		simpleFormat := vttJson.ToSimpleFormat()

		// Wrap each subtitle in its own tag
		filename := strings.TrimSuffix(sub.filename, ".vtt")
		subtitleContents = append(subtitleContents, fmt.Sprintf("<input_subtitle_%s>\n%s</input_subtitle_%s>", filename, simpleFormat, filename))
	}

	if len(subtitleContents) == 0 {
		return nil, fmt.Errorf("no subtitle files found in directory")
	}

	// Combine all subtitles into one prompt
	combinedSubtitles := strings.Join(subtitleContents, "\n\n")

	// Create prompt for translation with all subtitles
	prompt := fmt.Sprintf(`You are a professional subtitle translator. Translate the following subtitle files to %s.

The input subtitles are in this format:
delay: <seconds after previous subtitle>
duration: <duration in seconds>
<subtitle text>

Details to follow:
1. Translate ONLY the subtitle text to %s
2. Use the multiple language tracks as reference to understand context
3. Use consistent terminology across all subtitles (they belong to the same video)
4. Output your translation in the same format
5. Put your output inside <output></output> tags

Here are the subtitle files to translate (different language tracks from the same video):

%s

Output the translated subtitles to %s in the same format inside <output></output> tags.`, targetLanguage, targetLanguage, combinedSubtitles, targetLanguage)

	// Generate translation with streaming
	fmt.Println(prompt)
	iter := t.client.Models.GenerateContentStream(ctx, t.model, genai.Text(prompt), nil)

	var fullResponse strings.Builder
	for resp, err := range iter {
		if err != nil {
			return nil, fmt.Errorf("failed to generate translation: %w", err)
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				fmt.Print(part.Text) // Print to console for logging
				fullResponse.WriteString(part.Text)
				if streamCallback != nil {
					streamCallback(part.Text)
				}
			}
		}
	}

	translatedText := fullResponse.String()
	if translatedText == "" {
		return nil, fmt.Errorf("translation is empty")
	}

	// Extract content from <output> tags
	startTag := "<output>"
	endTag := "</output>"
	startIdx := strings.Index(translatedText, startTag)
	endIdx := strings.Index(translatedText, endTag)

	if startIdx == -1 || endIdx == -1 {
		return nil, fmt.Errorf("translation output not found in expected format (missing <output> tags)")
	}

	content := translatedText[startIdx+len(startTag) : endIdx]
	content = strings.TrimSpace(content)

	// Parse simple format output
	vttJson, err := subtitles.ParseSimpleFormat(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output: %w", err)
	}

	// Convert to WebVTT format
	vttContent := vttJson.ToWebVTTString()

	// Create output file path - save inside the subtitle directory
	// e.g., somepath/my_video/*.vtt -> somepath/my_video/targetLang.vtt
	outputPath := filepath.Join(exportedSubtitlesDir, fmt.Sprintf("%s.vtt", targetLanguage))

	// Write translated subtitle
	if err := os.WriteFile(outputPath, []byte(vttContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write translated subtitle: %w", err)
	}

	translatedFiles = append(translatedFiles, outputPath)

	return translatedFiles, nil
}
