package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	settingsFileName = "settings.json"
)

func getDefaultSettings() Settings {
	return Settings{
		SubtitleBurnInDefault:      true,
		DefaultTranslationLanguage: "English",
		GeminiApiKey:               "",
		GeminiModel:                "gemini-2.5-flash",
		DefaultQuality:             "original",
		SubtitleFontSize:           24,
	}
}

type Settings struct {
	SubtitleBurnInDefault      bool   `json:"subtitleBurnInDefault"`
	DefaultTranslationLanguage string `json:"defaultTranslationLanguage"`
	GeminiApiKey               string `json:"geminiApiKey"`
	GeminiModel                string `json:"geminiModel"`
	DefaultQuality             string `json:"defaultQuality"`
	SubtitleFontSize           int    `json:"subtitleFontSize"`
}

type SettingsStore struct {
	settings Settings
	filePath string
	ctx      context.Context
	mu       sync.RWMutex
}

func NewSettingsStore() *SettingsStore {
	// Get config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}

	appConfigDir := filepath.Join(configDir, "wails-cast")
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
