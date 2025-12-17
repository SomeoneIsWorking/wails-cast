package download

import (
	"context"
	"fmt"
	"wails-cast/pkg/cast"
	"wails-cast/pkg/options"
)

func (d *DownloadManager) GetStatus(url string, mediaType string, track int) (*DownloadStatus, error) {
	key := getKey(url, mediaType, track)
	task, exists := d.Items[key]
	if !exists {
		manifest, err := d.GetItem(url, mediaType, track)
		if err != nil {
			return nil, err
		}
		return &DownloadStatus{
			Url:       url,
			MediaType: mediaType,
			Track:     track,
			Progress:  manifest.Progress,
			Status:    "IDLE",
		}, nil
	}
	return &DownloadStatus{
		Url:       task.URL,
		MediaType: task.MediaType,
		Track:     task.Track,
		Progress:  task.Progress,
		Status:    task.Status,
	}, nil
}

func (d *DownloadManager) StartDownload(url string, mediaType string, index int) (*DownloadStatus, error) {
	// Create video task if video track is specified
	manifest, err := d.GetItem(url, mediaType, index)
	if err != nil {
		return nil, err
	}

	if manifest.Status == "INPROGRESS" {
		return nil, fmt.Errorf("download already in progress")
	}
	return d.startDownload(manifest)
}

func (d *DownloadManager) Stop(url string, mediaType string, track int) (*DownloadStatus, error) {
	task, err := d.GetItem(url, mediaType, track)
	if err != nil {
		return nil, err
	}
	if task.Status != "INPROGRESS" {
		return nil, fmt.Errorf("no download in progress to stop")
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.Cancel()
	<-task.Channel
	task.Status = "STOPPED"
	return &DownloadStatus{
		Url:       task.URL,
		MediaType: task.MediaType,
		Progress:  task.Progress,
		Track:     task.Track,
		Status:    task.Status,
	}, nil
}

func (d *DownloadManager) startDownload(task *DownloadItem) (*DownloadStatus, error) {
	if task.Handler == nil {
		var err error
		task.TrackInfo, err = cast.GetRemoteTrackInfo(task.URL)
		if err != nil {
			return nil, err
		}
		task.Handler, err = cast.CreateRemoteHandler(task.URL, options.StreamOptions{})

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
		Url:       task.URL,
		MediaType: task.MediaType,
		Track:     task.Track,
		Progress:  task.Progress,
		Status:    task.Status,
	}, nil
}

func startDownloadTask(context context.Context, task *DownloadItem) {
	// Signal completion if this ends in any way (error, cancellation, or full download)
	defer func() {
		select {
		case task.Channel <- "DONE":
		default:
		}
	}()

	for i, downloaded := range task.Progress {
		if downloaded {
			continue
		}
		if context.Err() != nil {
			return
		}
		_, err := task.Handler.EnsureSegmentDownloaded(context, task.MediaType, task.Track, i)
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
	}
	task.mu.Lock()
	task.Status = "JUST-COMPLETED"
	task.Emit()
	task.mu.Unlock()
}
