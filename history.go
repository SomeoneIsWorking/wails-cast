package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wails-cast/pkg/events"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/options"
)

const (
	maxHistoryItems = 50
	historyFileName = "cast_history.json"
)

type HistoryItem struct {
	FileNameOrUrl string               `json:"path"`
	Name          string               `json:"name"`
	Timestamp     string               `json:"timestamp"`
	CastOptions   *options.CastOptions `json:"castOptions"`
}

type HistoryStore struct {
	items    []HistoryItem
	filePath string
	mu       sync.RWMutex
}

func NewHistoryStore() *HistoryStore {
	appConfigDir := folders.GetConfig()
	os.MkdirAll(appConfigDir, 0755)

	historyPath := filepath.Join(appConfigDir, historyFileName)

	store := &HistoryStore{
		items:    []HistoryItem{},
		filePath: historyPath,
	}

	store.load()
	return store
}

func (h *HistoryStore) load() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(h.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history file yet
		}
		return err
	}

	return json.Unmarshal(data, &h.items)
}

func (h *HistoryStore) save() error {
	data, err := json.MarshalIndent(h.items, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.filePath, data, 0644)
}

func (h *HistoryStore) Add(fileNameOrUrl string, name string, castOptions *options.CastOptions) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	item := HistoryItem{
		FileNameOrUrl: fileNameOrUrl,
		Name:          name,
		Timestamp:     time.Now().Format(time.RFC3339),
		CastOptions:   castOptions,
	}

	// Remove duplicate if exists
	filtered := []HistoryItem{}
	for _, existing := range h.items {
		if existing.FileNameOrUrl != fileNameOrUrl {
			filtered = append(filtered, existing)
		}
	}

	// Add to beginning
	h.items = append([]HistoryItem{item}, filtered...)

	// Limit size
	if len(h.items) > maxHistoryItems {
		h.items = h.items[:maxHistoryItems]
	}

	err := h.save()
	if err == nil {
		events.Emit("history:updated", item)
	}
	return err
}

func (h *HistoryStore) GetAll() []HistoryItem {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a copy
	items := make([]HistoryItem, len(h.items))
	copy(items, h.items)
	return items
}

func (h *HistoryStore) Remove(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	filtered := []HistoryItem{}
	for _, item := range h.items {
		if item.FileNameOrUrl != path {
			filtered = append(filtered, item)
		}
	}

	h.items = filtered
	return h.save()
}

func (h *HistoryStore) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.items = []HistoryItem{}
	return h.save()
}
