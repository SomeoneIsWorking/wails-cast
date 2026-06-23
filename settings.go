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
		GeminiApiKey:               "",
		GeminiModel:                "deepseek-v4-flash",
		LLMProvider:                "opencode",
		OpenAICompatBaseURL:        "",
		OpenAICompatAPIKey:         "",
		OpenAICompatModel:          "",
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

	// GeminiApiKey / GeminiModel are the opencode-specific credentials.
	// The field names are kept as-is for backward-compatibility with existing
	// persisted settings.json files (renaming would silently lose saved values).
	GeminiApiKey string `json:"geminiApiKey"`
	GeminiModel  string `json:"geminiModel"`

	// LLMProvider selects which backend to use for AI features.
	// Supported values: "opencode" (default), "openai-compat".
	LLMProvider string `json:"llmProvider"`

	// OpenAICompatBaseURL / OpenAICompatAPIKey / OpenAICompatModel are used when
	// LLMProvider == "openai-compat".
	OpenAICompatBaseURL string `json:"openAICompatBaseURL"`
	OpenAICompatAPIKey  string `json:"openAICompatApiKey"`
	OpenAICompatModel   string `json:"openAICompatModel"`

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

	return json.Unmarshal(data, &s.settings)
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
