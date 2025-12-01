package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	maxHistoryItems = 50
	historyFileName = "cast_history.json"
)

type HistoryItem struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Timestamp  time.Time `json:"timestamp"`
	DeviceName string    `json:"deviceName"`
}

type HistoryStore struct {
	items    []HistoryItem
	filePath string
	ctx      context.Context
	mu       sync.RWMutex
}

func NewHistoryStore() *HistoryStore {
	// Get config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}

	appConfigDir := filepath.Join(configDir, "wails-cast")
	os.MkdirAll(appConfigDir, 0755)

	historyPath := filepath.Join(appConfigDir, historyFileName)

	store := &HistoryStore{
		items:    []HistoryItem{},
		filePath: historyPath,
	}

	store.load()
	return store
}

func (h *HistoryStore) SetContext(ctx context.Context) {
	h.ctx = ctx
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

func (h *HistoryStore) Add(path, deviceName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	name := filepath.Base(path)
	item := HistoryItem{
		Path:       path,
		Name:       name,
		Timestamp:  time.Now(),
		DeviceName: deviceName,
	}

	// Remove duplicate if exists
	filtered := []HistoryItem{}
	for _, existing := range h.items {
		if existing.Path != path {
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
	if err == nil && h.ctx != nil {
		wails_runtime.EventsEmit(h.ctx, "history:updated", item)
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
		if item.Path != path {
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
