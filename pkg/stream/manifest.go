package stream

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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
	Version     int                      `json:"version"`     // Schema version
	Items       map[string]*ManifestItem `json:"items"`       // URL -> Item
	SegmentMap  map[string]string        `json:"segment_map"` // ID -> URL
	URLMap      map[string]string        `json:"url_map"`     // URL -> ID
	NextID      int                      `json:"next_id"`
	AudioNextID int                      `json:"audio_next_id"`
	VideoNextID int                      `json:"video_next_id"`
	mu          sync.RWMutex
}

// getOrAssignID returns the ID for a URL, assigning a new one if necessary
func (p *RemoteHandler) getOrAssignID(url string, prefix string) string {
	p.ManifestData.mu.Lock()
	defer p.ManifestData.mu.Unlock()

	if id, ok := p.ManifestData.URLMap[url]; ok {
		return id
	}

	var id int
	switch prefix {
	case "audio_":
		id = p.ManifestData.AudioNextID
		p.ManifestData.AudioNextID++
	case "video_":
		id = p.ManifestData.VideoNextID
		p.ManifestData.VideoNextID++
	default:
		id = p.ManifestData.NextID
		p.ManifestData.NextID++
	}

	idStr := strconv.Itoa(id)
	p.ManifestData.URLMap[url] = idStr
	p.ManifestData.SegmentMap[idStr] = url

	// Create initial item if not exists
	if _, ok := p.ManifestData.Items[url]; !ok {
		p.ManifestData.Items[url] = &ManifestItem{
			URL: url,
		}
	}

	return idStr
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
