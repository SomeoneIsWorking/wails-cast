package main

import "wails-cast/pkg/remote"

type SubtitleDisplayItem struct {
	Path  string
	Label string
}

type AudioTracksDisplayItem struct {
	Index    int
	Language string
}

type VideoTrackDisplayItem struct {
	Index      int
	Codecs     string
	Resolution string
}

type TrackDisplayInfo struct {
	VideoTracks    []VideoTrackDisplayItem
	AudioTracks    []AudioTracksDisplayItem
	SubtitleTracks []SubtitleDisplayItem
	Path           string
	NearSubtitle   string
}

type QualityOption struct {
	Label   string
	Key     string
	Default bool
}

type AppExports struct {
	DownloadStatus remote.DownloadStatus
}

type PlaybackState struct {
	Status      string  `json:"status"`
	MediaPath   string  `json:"mediaPath"`
	MediaName   string  `json:"mediaName"`
	DeviceURL   string  `json:"deviceUrl"`
	DeviceName  string  `json:"deviceName"`
	CurrentTime float64 `json:"currentTime"`
	Duration    float64 `json:"duration"`
}
