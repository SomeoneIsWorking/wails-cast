package download

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"wails-cast/pkg/cast"
	"wails-cast/pkg/events"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/mediainfo"
	"wails-cast/pkg/options"
	"wails-cast/pkg/stream"
)

type DownloadManager struct {
	Items map[string]*DownloadItem
}

func NewDownloadManager() *DownloadManager {
	manager := DownloadManager{
		Items: make(map[string]*DownloadItem),
	}
	manager.setupListener()
	return &manager
}

type DownloadItem struct {
	Progress  []bool
	URL       string
	Status    string
	MediaType string // "video" or "audio"
	Track     int    // track index
	Handler   *stream.RemoteHandler
	Cancel    context.CancelFunc
	TrackInfo *mediainfo.MediaTrackInfo
	mu        sync.RWMutex
	Channel   chan string
}

func getKey(url string, mediaType string, track int) string {
	return fmt.Sprintf("%s|%s|%d", url, mediaType, track)
}

func (d *DownloadManager) GetItem(url string, mediaType string, track int) (*DownloadItem, error) {
	key := getKey(url, mediaType, track)
	storage, exists := d.Items[key]
	if exists {
		return storage, nil
	}

	manifest, err := getTrackProgress(url, mediaType, track)
	if err != nil {
		return nil, err
	}
	storage = &DownloadItem{
		Progress:  manifest,
		URL:       url,
		Status:    "IDLE",
		Track:     track,
		MediaType: mediaType,
		Channel:   make(chan string),
		mu:        sync.RWMutex{},
	}
	d.Items[key] = storage
	return d.Items[key], nil
}

// getTrackProgress returns the current download progress for a specific track
func getTrackProgress(url string, mediaType string, track int) ([]bool, error) {
	trackDir := filepath.Join(folders.GetCacheForVideo(url), fmt.Sprintf("%s_%d", mediaType, track))
	manifestFile := filepath.Join(trackDir, "download.json")
	manifest, err := readManifestFile(manifestFile)
	if err == nil {
		return manifest, nil
	}

	handler, err := cast.CreateRemoteHandler(url, options.StreamOptions{})
	if err != nil {
		return nil, err
	}

	playlist, err := handler.GetTrackPlaylist(context.Background(), mediaType, track)
	if err != nil {
		return nil, err
	}

	totalSegments := len(playlist.Segments)
	if totalSegments == 0 {
		return nil, nil
	}

	manifest = make([]bool, totalSegments)

	for i := range playlist.Segments {
		rawPath := filepath.Join(trackDir, fmt.Sprintf("segment_%d_raw.ts", i))
		if _, err := os.Stat(rawPath); err == nil {
			manifest[i] = true
		}
	}

	err = writeManifestFile(manifestFile, manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func writeManifestFile(manifestFile string, manifest []bool) error {
	os.MkdirAll(filepath.Dir(manifestFile), os.ModePerm)
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	err = os.WriteFile(manifestFile, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func readManifestFile(manifestFile string) ([]bool, error) {
	if _, err := os.Stat(manifestFile); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(manifestFile)
	if err != nil {
		return nil, err
	}

	var manifest []bool
	err = json.Unmarshal(data, &manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func (d *DownloadManager) setupListener() {
	events.Download.Subscribe(func(topic string, data any) {
		status, ok := data.(stream.DownloadReport)
		if !ok {
			return
		}
		item, err := d.GetItem(status.URL, status.MediaType, status.Track)
		if err != nil {
			return
		}
		item.mu.Lock()
		item.Progress[status.Segment] = true
		item.Emit()
		item.mu.Unlock()
		trackDir := filepath.Join(folders.GetCacheForVideo(status.URL), fmt.Sprintf("%s_%d", status.MediaType, status.Track))
		manifestFile := filepath.Join(trackDir, "download.json")
		writeManifestFile(manifestFile, item.Progress)
	})
}

// StopAllAndClear stops all active downloads, emits final states, and clears the manager map
func (d *DownloadManager) StopAllAndClear() error {
	for _, item := range d.Items {
		if item.Status == "INPROGRESS" {
			item.Cancel()
			<-item.Channel
		}
		item.mu.Lock()
		item.Status = "IDLE"
		item.Progress = make([]bool, len(item.Progress))
		item.Emit()
		item.mu.Unlock()
	}
	return nil
}

func (task *DownloadItem) Emit() {
	events.Emit("download:progress", task.ToStatus())
}

func (task *DownloadItem) ToStatus() DownloadStatus {
	return DownloadStatus{
		Url:       task.URL,
		MediaType: task.MediaType,
		Track:     task.Track,
		Progress:  task.Progress,
		Status:    task.Status,
	}
}

type DownloadStatus struct {
	Url       string
	MediaType string
	Track     int
	Progress  []bool
	Total     int
	Status    string
}
