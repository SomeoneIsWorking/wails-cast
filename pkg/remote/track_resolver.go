package remote

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"wails-cast/pkg/cache"
	"wails-cast/pkg/hls"
)

type TrackResolver struct {
	FileDownloader   *FileDownloader
	cacheChannel     chan int
	Manifest         *hls.ManifestPlaylist
	ManifestURL      *url.URL
	Folder           string
	Cache            bool
	TrackType        string
	TrackIndex       int
	StorageDirectory string
}

func (this *TrackResolver) trackUrl() (*url.URL, error) {
	var url *url.URL
	switch this.TrackType {
	case "video":
		if this.TrackIndex < 0 || this.TrackIndex >= len(this.Manifest.VideoTracks) {
			return nil, fmt.Errorf("video track index out of range")
		}
		url = this.Manifest.VideoTracks[this.TrackIndex].URI
	case "audio":
		if this.TrackIndex < 0 || this.TrackIndex >= len(this.Manifest.AudioTracks) {
			return nil, fmt.Errorf("audio track index out of range")
		}
		url = this.Manifest.AudioTracks[this.TrackIndex].URI
	case "subtitle":
		if this.TrackIndex < 0 || this.TrackIndex >= len(this.Manifest.SubtitleTracks) {
			return nil, fmt.Errorf("subtitle track index out of range")
		}
		url = this.Manifest.SubtitleTracks[this.TrackIndex].URI
	default:
		return nil, nil
	}
	return this.ManifestURL.ResolveReference(url), nil
}

func (this *TrackResolver) GetPlaylist(ctx context.Context) (*hls.TrackPlaylist, *url.URL, error) {
	url, err := this.trackUrl()
	if err != nil {
		return nil, nil, err
	}
	data, err := cache.Get(
		filepath.Join(this.StorageDirectory, "playlist.m3u8"),
		func() ([]byte, error) {
			return this.FileDownloader.DownloadFile(ctx, url)
		},
	)
	if err != nil {
		return nil, nil, err
	}
	playlist, err := hls.ParseTrackPlaylist(string(data))
	if err != nil {
		return nil, nil, err
	}
	return playlist, url, nil
}
