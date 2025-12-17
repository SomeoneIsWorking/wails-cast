package remote

import (
	"path/filepath"
	"wails-cast/pkg/cache"
	"wails-cast/pkg/extractor"
	"wails-cast/pkg/folders"
)

type Manager struct {
	Items map[string]*MediaItem
}

type MediaItem struct {
	URL  string
	Data ExtractionData
}

type ExtractionData struct {
	HLS string
}

// NewManager creates a new remote manager
func NewManager() *Manager {
	return &Manager{
		Items: make(map[string]*MediaItem),
	}
}

func (m *Manager) Get(url string) (*MediaItem, error) {
	if item, exists := m.Items[url]; exists {
		return item, nil
	}
	videoFolder := folders.Video(url)
	extractionFile := filepath.Join(videoFolder, "extraction.json")
	_, err := cache.GetJson(extractionFile, func() (*extractor.ExtractResult, error) {
		return extractor.ExtractManifestPlaylist(url)
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}
