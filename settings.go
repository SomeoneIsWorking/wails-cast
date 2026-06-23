package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"wails-cast/pkg/folders"
)

const (
	settingsFileName = "settings.json"
)

func getDefaultSettings() Settings {
	return Settings{
		SubtitleBurnIn:             true,
		IgnoreClosedCaptions:       false,
		DefaultTranslationLanguage: "English",
		LLMProvider:                "opencode",
		LLMApiKey:                  "",
		LLMModel:                   "",
		LLMBaseURL:                 "",
		DefaultQuality:             "5M",
		SubtitleFontSize:           24,
		MaxOutputWidth:             0,
		TranslatePromptTemplate:    "Create a subtitle translation in {{.TargetLanguage}} based on the references in other languages.\nMultiple language tracks from the same video are provided as reference to help you understand context and maintain consistent terminology.\n\nInput format:\ndelay: <seconds>\nduration: <seconds>\n<text>\n\n{{.SubtitleContent}}\n\nOutput the translation in the same format inside <llm_output></llm_output> tags.",
		MaxSubtitleSamples:         4,
		NoTranscodeCache:           false,
		RemoteAPIEnabled:           false,
		RemoteAPIPort:              9999,
		RemoteAPIToken:             "",
	}
}

type Settings struct {
	SubtitleBurnIn             bool   `json:"subtitleBurnIn"`
	IgnoreClosedCaptions       bool   `json:"ignoreClosedCaptions"`
	DefaultTranslationLanguage string `json:"defaultTranslationLanguage"`

	// LLMProvider selects which backend to use for AI features.
	// Supported values: "opencode" (default), "openai-compat".
	LLMProvider string `json:"llmProvider"`

	// LLMApiKey is the API key for the selected provider.
	// For "opencode", if empty, falls back to ai.LoadOpenCodeAPIKey().
	// For "openai-compat", this is the Bearer token.
	LLMApiKey string `json:"llmApiKey"`

	// LLMModel is the model to request from the provider.
	LLMModel string `json:"llmModel"`

	// LLMBaseURL is the base URL for the provider endpoint.
	// Only used when LLMProvider == "openai-compat".
	// For "opencode", the fixed ai.OpenCodeBaseURL is used instead.
	LLMBaseURL string `json:"llmBaseURL"`

	DefaultQuality          string `json:"defaultQuality"`
	SubtitleFontSize        int    `json:"subtitleFontSize"`
	MaxOutputWidth          int    `json:"maxOutputWidth"`
	TranslatePromptTemplate string `json:"translatePromptTemplate"`
	MaxSubtitleSamples      int    `json:"maxSubtitleSamples"`
	NoTranscodeCache        bool   `json:"noTranscodeCache"`

	// Library feature settings.
	LibraryRoot string `json:"libraryRoot"`
	TMDBApiKey  string `json:"tmdbApiKey"`

	// Remote API (HTTP server for companion apps, e.g. Android)
	RemoteAPIEnabled bool   `json:"remoteApiEnabled"`
	RemoteAPIPort    int    `json:"remoteApiPort"`
	RemoteAPIToken   string `json:"remoteApiToken"` // empty = no auth required
}

// legacySettings is used during load to migrate old field names to the new
// unified LLM fields. We unmarshal into this struct first, then copy any
// non-empty legacy values into Settings if the new fields are still empty.
type legacySettings struct {
	GeminiApiKey        string `json:"geminiApiKey"`
	GeminiModel         string `json:"geminiModel"`
	OpenAICompatBaseURL string `json:"openAICompatBaseURL"`
	OpenAICompatAPIKey  string `json:"openAICompatApiKey"`
	OpenAICompatModel   string `json:"openAICompatModel"`
}

type SettingsStore struct {
	settings Settings
	filePath string
	ctx      context.Context
	mu       sync.RWMutex
}

func NewSettingsStore() *SettingsStore {
	appConfigDir := folders.GetConfig()
	os.MkdirAll(appConfigDir, 0755)

	settingsPath := filepath.Join(appConfigDir, settingsFileName)

	store := &SettingsStore{
		settings: getDefaultSettings(),
		filePath: settingsPath,
	}

	store.load()
	return store
}

func (s *SettingsStore) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *SettingsStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No settings file yet, use defaults
		}
		return err
	}

	if err := json.Unmarshal(data, &s.settings); err != nil {
		return err
	}

	// Migration: if the new unified LLM fields are empty but the legacy fields
	// are present in the file, copy them over so we don't silently lose the
	// user's saved credentials. The next save will persist only the new names.
	var legacy legacySettings
	if err := json.Unmarshal(data, &legacy); err == nil {
		if s.settings.LLMApiKey == "" {
			switch s.settings.LLMProvider {
			case "openai-compat":
				s.settings.LLMApiKey = legacy.OpenAICompatAPIKey
			default:
				s.settings.LLMApiKey = legacy.GeminiApiKey
			}
		}
		if s.settings.LLMModel == "" {
			switch s.settings.LLMProvider {
			case "openai-compat":
				s.settings.LLMModel = legacy.OpenAICompatModel
			default:
				s.settings.LLMModel = legacy.GeminiModel
			}
		}
		if s.settings.LLMBaseURL == "" && s.settings.LLMProvider == "openai-compat" {
			s.settings.LLMBaseURL = legacy.OpenAICompatBaseURL
		}
	}

	return nil
}

func (s *SettingsStore) save() error {
	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *SettingsStore) Get() *Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &s.settings
}

func (s *SettingsStore) Update(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings = settings
	return s.save()
}

func (s *SettingsStore) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings = getDefaultSettings()
	return s.save()
}
