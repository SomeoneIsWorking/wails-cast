package main

import (
	"wails-cast/pkg/castapi"
	"wails-cast/pkg/remote"
)

type SubtitleDisplayItem = castapi.SubtitleDisplayItem
type AudioTracksDisplayItem = castapi.AudioTracksDisplayItem
type VideoTrackDisplayItem = castapi.VideoTrackDisplayItem
type TrackDisplayInfo = castapi.TrackDisplayInfo
type QualityOption = castapi.QualityOption
type PlaybackState = castapi.PlaybackState

type AppExports struct {
	DownloadStatus remote.DownloadStatus
}
