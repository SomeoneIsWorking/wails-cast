package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// OpenCodeBaseURL is the OpenAI-compatible endpoint for the "opencode-go" provider.
// This is the OpenCode Zen endpoint, which serves both the paid models and the
// free-tier models (e.g. "deepseek-v4-flash-free"). The "/zen/go/v1" variant only
// exposes the paid coding models and rejects the free model ids.
const OpenCodeBaseURL = "https://opencode.ai/zen/v1"

// openCodeProvider is the provider key looked up in opencode's auth.json.
const openCodeProvider = "opencode-go"

// opencodeAuthEntry mirrors a single provider entry in opencode's auth.json.
type opencodeAuthEntry struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// opencodeAuthPath returns the path to opencode's auth.json, honoring XDG_DATA_HOME.
func opencodeAuthPath() (string, error) {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "opencode", "auth.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "opencode", "auth.json"), nil
}

// LoadOpenCodeAPIKey reads the API key for the "opencode-go" provider from opencode's
// auth.json, so the translator stays in sync with the user's opencode config.
func LoadOpenCodeAPIKey() (string, error) {
	path, err := opencodeAuthPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read opencode auth (%s): %w", path, err)
	}

	var entries map[string]opencodeAuthEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return "", fmt.Errorf("failed to parse opencode auth: %w", err)
	}

	if entry, ok := entries["opencode"]; ok && entry.Key != "" {
		return entry.Key, nil
	}

	return "", fmt.Errorf("no opencode API key found in %s", path)
}
