package stream

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// ManifestItem represents a cached file
type ManifestItem struct {
	URL            string `json:"url"`
	ContentType    string `json:"content_type"`
	LocalPath      string `json:"local_path"`
	TranscodedPath string `json:"transcoded_path,omitempty"`
	Transcoded     bool   `json:"transcoded"`
	IsPlaylist     bool   `json:"is_playlist"`
}

const CurrentManifestVersion = 1

// Manifest represents the tracking of cached files
type Manifest struct {
	Version int                      `json:"version"` // Schema version
	Items   map[string]*ManifestItem `json:"items"`   // URL -> Item
	mu      sync.RWMutex
}

// saveManifest saves the manifest to disk
func (p *RemoteHandler) saveManifest() {
	p.ManifestData.mu.Lock()
	defer p.ManifestData.mu.Unlock()

	data, err := json.MarshalIndent(p.ManifestData, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling manifest: %v\n", err)
		return
	}

	err = os.WriteFile(p.ManifestPath, data, 0644)
	if err != nil {
		fmt.Printf("Error saving manifest: %v\n", err)
	}
}

// updateManifest updates the manifest with a new item
func (p *RemoteHandler) updateManifest(url string, item *ManifestItem) {
	p.ManifestData.mu.Lock()
	p.ManifestData.Items[url] = item
	p.ManifestData.mu.Unlock()
	p.saveManifest()
}
