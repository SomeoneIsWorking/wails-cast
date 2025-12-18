package remote

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"wails-cast/pkg/filehelper"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/mix"

	"github.com/pkg/errors"
)

type TrackManager struct {
	FileDownloader     FileDownloader
	cacheChannel       chan int
	Manifest           *hls.TrackPlaylist
	ManifestURL        *url.URL
	Folder             string
	Cache              bool
	TrackType          string
	TrackIndex         int
	StorageDirectory   string
	DownloadStatus     string
	DownloadedSegments []bool
}

type DownloadStatusQeuryResponse struct {
	Status   string
	Segments []bool
}

func (this *TrackManager) StopDownload() error {
	panic("unimplemented")
}

func (this *TrackManager) StartDownload() error {
	panic("unimplemented")
}

func (this *TrackManager) GetDownloadStatus() *DownloadStatusQeuryResponse {
	return &DownloadStatusQeuryResponse{
		Status:   this.DownloadStatus,
		Segments: this.DownloadedSegments,
	}
}

func (this *TrackManager) StopAndClear() error {
	panic("unimplemented")
}

func (this *TrackManager) GetDuration() float64 {
	var totalDuration float64
	for _, segment := range this.Manifest.Segments {
		totalDuration += segment.Duration
	}
	return totalDuration
}

func NewTrackManager(
	fileDownloader FileDownloader,
	manifest *hls.TrackPlaylist,
	manifestURL *url.URL,
	folder string,
	cache bool,
	trackType string,
	trackIndex int,
	storageDirectory string,
	cacheChannel chan int,
) *TrackManager {
	return &TrackManager{
		FileDownloader:     fileDownloader,
		Manifest:           manifest,
		ManifestURL:        manifestURL,
		Folder:             folder,
		Cache:              cache,
		TrackType:          trackType,
		TrackIndex:         trackIndex,
		StorageDirectory:   storageDirectory,
		DownloadedSegments: computeDownloadedSegments(folder, len(manifest.Segments)),
		cacheChannel:       cacheChannel,
		DownloadStatus:     "IDLE",
	}
}

func computeDownloadedSegments(folder string, total int) []bool {
	downloaded := make([]bool, total)
	for i := range total {
		segmentPath := getSegmentPath(folder, i)
		if filehelper.Exists(segmentPath) {
			downloaded[i] = true
		}
	}
	return downloaded
}

func (this *TrackManager) downloadSegment(ctx context.Context, segmentIndex int) ([]byte, error) {
	url, err := this.resolveSegmentURL(segmentIndex)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve segment URL for index: %d", segmentIndex)
	}
	data, err := this.FileDownloader.DownloadFile(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download segment: %s", url)
	}

	return data, nil
}

func (this *TrackManager) resolveSegmentURL(segmentIndex int) (*url.URL, error) {
	segment := this.Manifest.Segments[segmentIndex]
	resolvedUrl := this.ManifestURL.ResolveReference(segment.URI)
	return resolvedUrl, nil
}

func (this *TrackManager) GetSegment(ctx context.Context, segmentIndex int) (*mix.FileOrBuffer, error) {
	cachePath := getSegmentPath(this.Folder, segmentIndex)

	if _, err := os.Stat(cachePath); err == nil {
		return mix.File(cachePath), nil
	}

	data, err := this.downloadSegment(ctx, segmentIndex)
	if err != nil {
		return nil, err
	}

	if !this.Cache {
		return mix.Buffer(data), nil
	}

	if filehelper.WriteFile(cachePath, data) == nil {
		this.DownloadedSegments[segmentIndex] = true
		this.statusUpdate()
	}
	return mix.File(cachePath), nil
}

func (this *TrackManager) statusUpdate() {
	select {
	case this.cacheChannel <- 0:
	default:
	}
}

func getSegmentPath(folder string, segmentIndex int) string {
	return filepath.Join(folder, fmt.Sprintf("segment_%d_raw.ts", segmentIndex))
}
