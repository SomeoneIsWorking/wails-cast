package download

import (
	"context"
	"fmt"
	"sync"
	"wails-cast/pkg/cast"
	"wails-cast/pkg/events"
	"wails-cast/pkg/mediainfo"
	"wails-cast/pkg/options"
	"wails-cast/pkg/stream"
)

type DownloadManager struct {
	Downloads map[string]*DownloadTask
}

func NewDownloadManager() *DownloadManager {
	return &DownloadManager{
		Downloads: make(map[string]*DownloadTask),
	}
}

func (d *DownloadManager) GetStatus(url string, mediaType string, track int) (*DownloadStatus, error) {
	key := getKey(url, mediaType, track)
	task, exists := d.Downloads[key]
	if !exists {
		downloaded, total, err := cast.GetTrackProgress(url, mediaType, track)
		if err != nil {
			return nil, err
		}
		return &DownloadStatus{
			Url:        url,
			MediaType:  mediaType,
			Track:      track,
			Downloaded: downloaded,
			Total:      total,
			Status:     "IDLE",
		}, nil
	}
	return &DownloadStatus{
		Url:        task.URL,
		MediaType:  task.MediaType,
		Track:      task.Track,
		Downloaded: task.Downloaded,
		Total:      task.Total,
		Status:     task.Status,
	}, nil
}

func (d *DownloadManager) StartDownload(url string, mediaType string, index int) (*DownloadStatus, error) {
	// Create video task if video track is specified
	key := getKey(url, mediaType, index)
	if _, exists := d.Downloads[key]; !exists {
		downloaded, total, err := cast.GetTrackProgress(url, mediaType, index)
		if err != nil {
			return nil, err
		}
		status := "IDLE"
		if downloaded == total {
			status = "COMPLETED"
		}
		d.Downloads[key] = &DownloadTask{
			URL:        url,
			Status:     status,
			MediaType:  mediaType,
			Downloaded: downloaded,
			Total:      total,
			Track:      index,
			Channel:    make(chan string),
		}
	}
	if d.Downloads[key].Status == "INPROGRESS" {
		return nil, fmt.Errorf("download already in progress")
	}
	return d.startDownload(d.Downloads[key])
}

func (d *DownloadManager) Stop(url string, mediaType string, track int) (*DownloadStatus, error) {
	key := getKey(url, mediaType, track)
	task, exists := d.Downloads[key]
	if !exists {
		return nil, fmt.Errorf("No such download found")
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.Cancel()
	<-task.Channel
	task.Status = "STOPPED"
	return &DownloadStatus{
		Url:        task.URL,
		MediaType:  task.MediaType,
		Track:      task.Track,
		Downloaded: task.Downloaded,
		Total:      task.Total,
		Status:     task.Status,
	}, nil
}

func getKey(url string, mediaType string, track int) string {
	return fmt.Sprintf("%s|%s|%d", url, mediaType, track)
}

func (d *DownloadManager) startDownload(task *DownloadTask) (*DownloadStatus, error) {
	if task.Handler == nil {
		var err error
		task.TrackInfo, err = cast.GetRemoteTrackInfo(task.URL)
		if err != nil {
			return nil, err
		}
		videoTrack := -1
		if task.MediaType == "video" {
			videoTrack = task.Track
		}
		audioTrack := -1
		if task.MediaType == "audio" {
			audioTrack = task.Track
		}
		task.Handler, err = cast.CreateRemoteHandler(task.URL, &options.StreamOptions{
			VideoTrack: videoTrack,
			AudioTrack: audioTrack,
		})

		if err != nil {
			return nil, err
		}
	}
	task.mu.Lock()
	context, cancel := context.WithCancel(context.Background())
	task.Cancel = cancel
	task.Status = "INPROGRESS"
	task.mu.Unlock()

	go startDownloadTask(context, task)

	return &DownloadStatus{
		Url:        task.URL,
		MediaType:  task.MediaType,
		Track:      task.Track,
		Downloaded: task.Downloaded,
		Total:      task.Total,
		Status:     task.Status,
	}, nil
}

func startDownloadTask(context context.Context, task *DownloadTask) {
	// Signal completion if this ends in any way (error, cancellation, or full download)
	defer func() {
		select {
		case task.Channel <- "DONE":
		default:
		}
	}()

	for task.Downloaded < task.Total {
		if context.Err() != nil {
			return
		}
		_, err := task.Handler.EnsureSegmentDownloaded(context, task.MediaType, task.Track, task.Downloaded)
		if context.Err() != nil {
			return
		}
		if err != nil {
			task.mu.Lock()
			task.Status = "ERROR"
			task.Emit()
			task.mu.Unlock()
			return
		}
		task.mu.Lock()
		task.Downloaded++
		task.Emit()
		task.mu.Unlock()
	}
	task.mu.Lock()
	task.Status = "JUST-COMPLETED"
	task.Emit()
	task.mu.Unlock()
}

// StopAllAndClear stops all active downloads, emits final states, and clears the manager map
func (d *DownloadManager) StopAllAndClear() error {
	for _, task := range d.Downloads {
		if task.Status == "INPROGRESS" {
			task.Cancel()
			<-task.Channel
		}
		task.mu.Lock()
		task.Status = "IDLE"
		task.Downloaded = 0
		task.Emit()
		task.mu.Unlock()
	}
	return nil
}

func (task *DownloadTask) Emit() {
	events.Emit("download:progress", DownloadStatus{
		Url:        task.URL,
		MediaType:  task.MediaType,
		Track:      task.Track,
		Downloaded: task.Downloaded,
		Total:      task.Total,
		Status:     task.Status,
	})
}

type DownloadStatus struct {
	Url        string
	MediaType  string
	Track      int
	Downloaded int
	Total      int
	Status     string
}

type DownloadTask struct {
	URL        string
	Downloaded int
	Total      int
	Status     string
	MediaType  string // "video" or "audio"
	Track      int    // track index
	Handler    *stream.RemoteHandler
	Cancel     context.CancelFunc
	TrackInfo  *mediainfo.MediaTrackInfo
	mu         sync.RWMutex
	Channel    chan string
}
