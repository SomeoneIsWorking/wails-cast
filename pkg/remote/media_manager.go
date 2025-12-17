package remote

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"wails-cast/pkg/hls"
)

type MediaManager struct {
	Title          string
	Items          map[string]*TrackManager
	FileDownloader *FileDownloader
	Manifest       *hls.ManifestPlaylist
	ManifestURL    *url.URL
	RootDir        string
	Cache          bool
}

func (this *MediaManager) StartDownload(mediaType string, index int) (*DownloadStatus, error) {
	trackManager, err := this.GetTrack(context.Background(), mediaType, index)
	if err != nil {
		return nil, err
	}
	return trackManager.StartDownload()
}

func (this *MediaManager) StopDownload(mediaType string, index int) (*DownloadStatus, error) {
	trackManager, err := this.GetTrack(context.Background(), mediaType, index)
	if err != nil {
		return nil, err
	}
	return trackManager.StopDownload()
}

func (this *MediaManager) GetDownloadStatus(mediaType string, track int) (*DownloadStatus, error) {
	trackManager, err := this.GetTrack(context.Background(), mediaType, track)
	if err != nil {
		return nil, err
	}
	return trackManager.GetDownloadStatus()
}

func (this *MediaManager) StopAllAndClear() error {
	for _, item := range this.Items {
		err := item.StopAndClear()
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *MediaManager) GetDuration() float64 {
	trackManager, err := this.GetTrack(context.Background(), "video", 0)
	if err != nil {
		return 0
	}
	return trackManager.GetDuration()
}

func NewMediaManager(
	rootDir string,
	title string,
	manifestUrl *url.URL,
	manifest *hls.ManifestPlaylist,
	fileDownloader *FileDownloader,
	cache bool,
) *MediaManager {
	return &MediaManager{
		RootDir:        rootDir,
		Items:          make(map[string]*TrackManager),
		FileDownloader: fileDownloader,
		Manifest:       manifest,
		ManifestURL:    manifestUrl,
		Title:          title,
		Cache:          cache,
	}
}

func (this *MediaManager) GetTrack(
	ctx context.Context,
	trackType string,
	trackIndex int,
) (*TrackManager, error) {
	key := fmt.Sprintf("%s_%d", trackType, trackIndex)
	if item, exists := this.Items[key]; exists {
		return item, nil
	}

	trackResolver := this.createTrackResolver(trackType, trackIndex)

	trackManifest, trackURL, err := trackResolver.GetPlaylist(ctx)
	if err != nil {
		return nil, err
	}

	trackManager := NewTrackManager(
		*this.FileDownloader,
		trackManifest,
		trackURL,
		filepath.Join(this.RootDir, key),
		this.Cache,
		trackType,
		trackIndex,
		filepath.Join(this.RootDir, key),
	)
	this.Items[key] = trackManager
	return trackManager, nil
}

func (this *MediaManager) createTrackResolver(trackType string, trackIndex int) *TrackResolver {
	trackResolver := &TrackResolver{
		Manifest:         this.Manifest,
		ManifestURL:      this.ManifestURL,
		TrackType:        trackType,
		TrackIndex:       trackIndex,
		StorageDirectory: filepath.Join(this.RootDir, fmt.Sprintf("%s_%d", trackType, trackIndex)),
		FileDownloader:   this.FileDownloader,
	}
	return trackResolver
}
