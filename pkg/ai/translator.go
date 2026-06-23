package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"wails-cast/pkg/events"
	"wails-cast/pkg/subtitles"
)

// TranslateOptions contains options for subtitle translation
type TranslateOptions struct {
	ExportedSubtitlesDir string
	TargetLanguage       string
	PromptTemplate       string
	MaxSubtitleSamples   int
}

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

// Translator handles AI-powered subtitle translation via the opencode-go
// (OpenAI-compatible) gateway.
type Translator struct {
	client  *http.Client
	apiKey  string
	model   string
	baseURL string
}

// NewTranslator creates a new translator instance backed by opencode-go.
func NewTranslator(apiKey string, model string) (*Translator, error) {
	return NewTranslatorWithBaseURL(apiKey, model, OpenCodeBaseURL)
}

// NewTranslatorWithBaseURL creates a translator that talks to any
// OpenAI-compatible endpoint.  Use this for the "openai-compat" provider.
func NewTranslatorWithBaseURL(apiKey, model, baseURL string) (*Translator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	return &Translator{
		client:  &http.Client{},
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
	}, nil
}

// StreamCompletion satisfies the LLMClient interface by delegating to the
// internal streaming implementation.
func (t *Translator) StreamCompletion(ctx context.Context, prompt string) (string, error) {
	return t.streamCompletion(ctx, prompt)
}

// Close releases any held resources.
func (t *Translator) Close() error {
	return nil
}

// chatMessage is a single OpenAI-compatible chat message.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the OpenAI-compatible chat completion request body.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// chatStreamChunk is a single SSE delta chunk from the chat completions stream.
type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// streamCompletion sends the prompt to the chat completions endpoint and streams
// the response back, emitting each chunk and returning the accumulated text.
func (t *Translator) streamCompletion(ctx context.Context, prompt string) (string, error) {
	reqBody, err := json.Marshal(chatRequest{
		Model:    t.model,
		Messages: []chatMessage{{Role: "user", Content: prompt}},
		Stream:   true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call opencode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("opencode returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed/keep-alive lines
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fmt.Print(choice.Delta.Content) // Print to console for logging
				fullResponse.WriteString(choice.Delta.Content)
				events.Emit("translation:stream", choice.Delta.Content)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read stream: %w", err)
	}

	return fullResponse.String(), nil
}

// TranslateEmbeddedSubtitles exports and translates all embedded subtitles
func (t *Translator) TranslateEmbeddedSubtitles(ctx context.Context, opts TranslateOptions) ([]string, error) {
	var translatedFiles []string

	// Read all .vtt files in the directory
	entries, err := os.ReadDir(opts.ExportedSubtitlesDir)
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

		subtitlePath := filepath.Join(opts.ExportedSubtitlesDir, entry.Name())
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
	if len(allSubtitles) > opts.MaxSubtitleSamples {
		allSubtitles = allSubtitles[:opts.MaxSubtitleSamples]
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

	tmpl, err := template.New("translate").Parse(opts.PromptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt template: %w", err)
	}

	var promptBuf bytes.Buffer
	err = tmpl.Execute(&promptBuf, map[string]string{
		"TargetLanguage":  opts.TargetLanguage,
		"SubtitleContent": combinedSubtitles,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute prompt template: %w", err)
	}

	prompt := promptBuf.String()

	// Generate translation with streaming
	fmt.Println(prompt)
	translatedText, err := t.streamCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate translation: %w", err)
	}

	if translatedText == "" {
		return nil, fmt.Errorf("translation is empty")
	}

	// Extract content from <llm_output> tags, or use full response if tags missing
	startTag := "<llm_output>"
	endTag := "</llm_output>"
	startIdx := strings.Index(translatedText, startTag)
	endIdx := strings.Index(translatedText, endTag)

	var content string
	if startIdx != -1 && endIdx != -1 {
		// Extract from tags
		content = translatedText[startIdx+len(startTag) : endIdx]
	} else {
		// Fallback: use entire response
		fmt.Println("Warning: <llm_output> tags not found, using entire response")
		content = translatedText
	}
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
	outputPath := filepath.Join(opts.ExportedSubtitlesDir, fmt.Sprintf("%s.vtt", opts.TargetLanguage))

	// Write translated subtitle
	if err := os.WriteFile(outputPath, []byte(vttContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write translated subtitle: %w", err)
	}

	translatedFiles = append(translatedFiles, outputPath)

	return translatedFiles, nil
}
