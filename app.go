package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vishen/go-chromecast/application"
	cast_proto "github.com/vishen/go-chromecast/cast/proto"
	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wails-cast/pkg/ai"
	localcast "wails-cast/pkg/cast"
	"wails-cast/pkg/download"
	"wails-cast/pkg/events"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	_logger "wails-cast/pkg/logger"
	"wails-cast/pkg/mediainfo"
	"wails-cast/pkg/options"
	"wails-cast/pkg/stream"
)

var logger = _logger.Logger
var customAppID = "7B88BB2E" // Custom receiver app ID

type SubtitleDisplayItem struct {
	Path  string `json:"path"`
	Label string `json:"label"`
}

type TrackDisplayInfo struct {
	VideoTracks    []mediainfo.VideoTrack `json:"videoTracks"`
	AudioTracks    []mediainfo.AudioTrack `json:"audioTracks"`
	SubtitleTracks []SubtitleDisplayItem  `json:"subtitleTracks"`
	Path           string                 `json:"path"`
	NearSubtitle   string                 `json:"nearSubtitle"`
}

type QualityOption struct {
	Label   string
	Key     string
	Default bool
}

type App struct {
	ctx             context.Context
	App             *application.Application
	discovery       *DeviceDiscovery
	mediaServer     *Server
	localIp         string
	playbackState   PlaybackState
	historyStore    *HistoryStore
	settingsStore   *SettingsStore
	mu              sync.RWMutex
	downloadManager *download.DownloadManager
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

func (a *App) createApplication() {
	a.stopPlayback(false) // ignore error
	app := application.NewApplication()
	app.AddMessageFunc(a.handleChromecastMessage)
	app.SetRequestTimeout(30 * time.Second)
	a.App = app
}

func NewApp() *App {
	discovery := NewDeviceDiscovery()
	localIP := discovery.GetLocalIP()

	return &App{
		discovery:       NewDeviceDiscovery(),
		mediaServer:     NewServer(localIP, 8888),
		localIp:         localIP,
		playbackState:   PlaybackState{},
		historyStore:    NewHistoryStore(),
		settingsStore:   NewSettingsStore(),
		downloadManager: download.NewDownloadManager(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	events.Subscribe(func(topic string, payload any) {
		wails_runtime.EventsEmit(a.ctx, topic, payload)
	})
	// Start media server
	go a.mediaServer.Start()

	logger.Info("App started")
}

func (a *App) shutdown(ctx context.Context) {
	a.mediaServer.Stop()
}

// DiscoverDevices discovers Chromecast devices on the network
func (a *App) DiscoverDevices() []Device {
	// Start streaming discovery and emit events for each device found
	err := a.discovery.DiscoverStream()
	if err != nil {
		logger.Error("Device discovery failed to start", "error", err)
	}
	// Return immediately; frontend will receive devices via events
	return []Device{}
}

// GetMediaURL returns the URL for a media file to be cast
func (a *App) GetMediaURL(filePath string) string {
	return fmt.Sprintf("http://%s:%d/playlist.m3u8", a.localIp, 8888)
}

// GetSubtitleURL returns the URL for subtitle file (for Shaka player)
func (a *App) GetSubtitleURL(subtitlePath string) string {
	if subtitlePath == "" {
		return ""
	}
	return "/subtitle.vtt"
}

func (a *App) GetTrackDisplayInfo(fileNameOrUrl string) (*TrackDisplayInfo, error) {
	trackInfo := &mediainfo.MediaTrackInfo{}
	var err error
	remote := strings.HasPrefix(fileNameOrUrl, "http://") || strings.HasPrefix(fileNameOrUrl, "https://")
	if remote {
		trackInfo, err = localcast.GetRemoteTrackInfo(fileNameOrUrl)
	} else {
		trackInfo, err = hls.GetMediaTrackInfo(fileNameOrUrl)
	}
	if err != nil {
		return nil, err
	}
	var subtitleItems = []SubtitleDisplayItem{
		{
			Path:  "none",
			Label: "None",
		},
		{
			Path:  "external",
			Label: "External",
		},
	}

	for _, sub := range trackInfo.SubtitleTracks {
		subtitleItems = append(subtitleItems, SubtitleDisplayItem{
			Path:  fmt.Sprintf("embedded:%d", sub.Index),
			Label: fmt.Sprintf("Embedded: %s", sub.Title),
		})
	}

	nearSubtitle := ""
	if !remote {
		nearSubtitle = findSubtitleFile(fileNameOrUrl)
	}

	return &TrackDisplayInfo{
		VideoTracks:    trackInfo.VideoTracks,
		AudioTracks:    trackInfo.AudioTracks,
		SubtitleTracks: subtitleItems,
		Path:           fileNameOrUrl,
		NearSubtitle:   nearSubtitle,
	}, nil
}

// CastToDevice casts media (local file or remote URL) to a device
func (a *App) CastToDevice(deviceIp string, fileNameOrUrl string, castOptions *options.CastOptions) (*PlaybackState, error) {
	// Determine if input is a local file or remote URL
	isRemote := strings.HasPrefix(fileNameOrUrl, "http://") || strings.HasPrefix(fileNameOrUrl, "https://")
	settings := a.GetSettings()
	options := options.StreamOptions{
		Subtitle: options.SubtitleCastOptions{
			Path:                 castOptions.SubtitlePath,
			BurnIn:               settings.SubtitleBurnIn,
			FontSize:             settings.SubtitleFontSize,
			IgnoreClosedCaptions: settings.IgnoreClosedCaptions,
		},
		VideoTrack:       castOptions.VideoTrack,
		AudioTrack:       castOptions.AudioTrack,
		Bitrate:          castOptions.Bitrate,
		MaxOutputWidth:   settings.MaxOutputWidth,
		NoTranscodeCache: settings.NoTranscodeCache,
	}
	var mediaPath string
	var duration float64
	var err error

	host := deviceIp
	port := 8009

	if isRemote {
		mediaPath = fileNameOrUrl
		// Use CastManager to prepare remote stream
		logger.Info("Preparing remote stream", "url", mediaPath)
		handler, err := localcast.CreateRemoteHandler(mediaPath, options)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare remote stream: %w", err)
		}
		duration = handler.Duration

		// Set handler on server
		a.mediaServer.SetHandler(handler)

	} else {
		mediaPath = fileNameOrUrl
		// Get duration for local files
		duration, err = hls.GetVideoDuration(mediaPath)
		if err != nil {
			logger.Warn("Failed to get duration", "error", err)
			duration = 0
		}

		// Create local handler
		handler := stream.NewLocalHandler(mediaPath, options)
		a.mediaServer.SetHandler(handler)

		// Set subtitle path (for legacy/compatibility, though options has it)
		a.mediaServer.SetSubtitlePath(options.Subtitle.Path)
	}

	// Update playback state
	a.mu.Lock()
	a.playbackState.MediaPath = mediaPath
	a.playbackState.MediaName = filepath.Base(mediaPath)
	a.playbackState.DeviceURL = deviceIp
	a.playbackState.DeviceName = extractDeviceName(deviceIp)
	a.playbackState.Duration = duration
	a.mu.Unlock()

	if deviceIp == "local" {
		// Just host the stream without casting
		logger.Info("Hosting stream without casting", "url", a.GetMediaURL(mediaPath))
		a.mu.Lock()
		a.playbackState.Status = "PLAYING"
		a.mu.Unlock()
		a.addToCastHistory(mediaPath, deviceIp, castOptions)
		return &a.playbackState, nil
	}

	mediaURL := a.GetMediaURL(mediaPath)

	a.createApplication()

	err = a.App.Start(host, port)

	if err != nil {
		return nil, err
	}

	err = a.App.Load(mediaURL+"?cachebust="+time.Now().Format("20060102150405"), application.LoadOptions{
		StartTime:   0,
		Transcode:   false,
		Detach:      true,
		ForceDetach: false,
		ContentType: "application/vnd.apple.mpegurl",
		AppId:       customAppID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load media on device: %w", err)
	}

	err = a.App.Update()
	if err != nil {
		return nil, fmt.Errorf("failed to update media status: %w", err)
	}

	logger.Info("Cast successful",
		"message", fmt.Sprintf("Casting %s to %s via %s", filepath.Base(mediaPath), deviceIp, mediaURL),
		"device", deviceIp,
		"media", mediaPath,
		"subtitle", options.Subtitle.Path,
	)

	// Add to history
	a.addToCastHistory(mediaPath, deviceIp, castOptions)

	return &a.playbackState, nil
}

func (a *App) addToCastHistory(mediaPath string, deviceIp string, castOptions *options.CastOptions) {
	if err := a.historyStore.Add(mediaPath, extractDeviceName(deviceIp), castOptions); err != nil {
		logger.Warn("Failed to add to history", "error", err)
	}
}

// GetMediaFiles returns media files from a directory
func (a *App) GetMediaFiles(dirPath string) ([]string, error) {
	var files []string
	extensions := map[string]bool{
		".mp4": true, ".mkv": true, ".webm": true,
		".avi": true, ".mov": true, ".flv": true,
		".m4v": true,
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if extensions[ext] {
				files = append(files, path)
			}
		}
		return nil
	})

	return files, err
}

func findSubtitleFile(videoPath string) string {
	baseName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	exts := []string{".srt", ".vtt", ".ass", ".ssa"}

	for _, ext := range exts {
		subPath := baseName + ext
		if _, err := os.Stat(subPath); err == nil {
			logger.Info("Found subtitle file", "path", subPath)
			return subPath
		}
	}

	return ""
}

// SeekTo seeks to a specific time
func (a *App) SeekTo(seekTime float64) error {
	// Send seek command to Chromecast
	err := a.App.SeekToTime(float32(seekTime))
	if err != nil {
		logger.Error("Seek failed", "error", err)
		return err
	}

	logger.Info("Seek successful", "time", seekTime)
	return nil
}

// UpdateSubtitleSettings updates subtitle settings for current media without recasting
func (a *App) UpdateSubtitleSettings(options options.SubtitleCastOptions) error {
	// Update subtitle path on server (clears cache)
	a.mediaServer.SetSubtitlePath(options.Path)
	a.App.Update()

	currentTime := a.playbackState.CurrentTime
	return a.App.SeekToTime(float32(currentTime))
}

func (a *App) ClearCache() error {
	return folders.DeleteAllCache()
}

// GetCacheStats returns cache statistics
func (a *App) GetCacheStats() (*folders.CacheStats, error) {
	return folders.GetCacheStats()
}

// DeleteTranscodedCache removes only transcoded video segments
func (a *App) DeleteTranscodedCache() error {
	if err := a.downloadManager.StopAllAndClear(); err != nil {
		return err
	}
	return folders.DeleteTranscodedCache()
}

// DeleteAllVideoCache removes all video files but keeps metadata
func (a *App) DeleteAllVideoCache() error {
	if err := a.downloadManager.StopAllAndClear(); err != nil {
		return err
	}
	return folders.DeleteAllVideoCache()
}

// OpenMediaFolder opens the folder containing the media file
func (a *App) OpenMediaFolder(fileNameOrUrl string) error {
	dir := folders.Video(fileNameOrUrl)
	os.MkdirAll(dir, 0755)
	cmd := exec.Command("open", dir)
	return cmd.Start()
}

func (a *App) StartDownload(url string, mediaType string, index int) (*download.DownloadStatus, error) {
	return a.downloadManager.StartDownload(url, mediaType, index)
}

func (a *App) StopDownload(url string, mediaType string, index int) (*download.DownloadStatus, error) {
	return a.downloadManager.Stop(url, mediaType, index)
}

// GetDownloadStatus returns the current download progress for a specific track
func (a *App) GetDownloadStatus(filenameOrUrl string, mediaType string, track int) (*download.DownloadStatus, error) {
	return a.downloadManager.GetStatus(filenameOrUrl, mediaType, track)
}

// Pause pauses current playback
func (a *App) Pause() error {
	err := a.App.Pause()
	if err != nil {
		return err
	}

	return nil
}

// Unpause resumes current playback
func (a *App) Unpause() error {
	err := a.App.Unpause()
	if err != nil {
		return err
	}

	return nil
}

// StopPlayback stops current playback
func (a *App) StopPlayback() error {
	err := a.stopPlayback(true)
	if err == nil {
		a.mu.Lock()
		a.playbackState.Status = "STOPPED"
		events.Emit("playback:state", a.playbackState)
		a.mu.Unlock()
	}
	return err
}

func (a *App) stopPlayback(stopMedia bool) error {
	if a.App != nil {
		err := a.App.Close(stopMedia)
		a.App = nil
		return err
	}
	return nil
}

// OpenFileDialog opens a file picker dialog
func (a *App) OpenFileDialog(title string, filters []string) (string, error) {
	if title == "" {
		title = "Select File"
	}

	// Convert filters to Wails format
	var wailsFilters []wails_runtime.FileFilter
	if len(filters) > 0 {
		pattern := strings.Join(filters, ";")
		wailsFilters = []wails_runtime.FileFilter{
			{
				DisplayName: "Files",
				Pattern:     pattern,
			},
		}
	}

	return wails_runtime.OpenFileDialog(a.ctx, wails_runtime.OpenDialogOptions{
		Title:   title,
		Filters: wailsFilters,
	})
}

// LogInfo logs an info message from frontend
func (a *App) LogInfo(message string, args ...any) {
	logger.Info(message, args...)
}

// LogWarn logs a warning message from frontend
func (a *App) LogWarn(message string, args ...any) {
	logger.Warn(message, args...)
}

// LogError logs an error message from frontend
func (a *App) LogError(message string, args ...any) {
	logger.Error(message, args...)
}

// ExportEmbeddedSubtitles exports all embedded subtitle tracks to WebVTT files
func (a *App) ExportEmbeddedSubtitles(videoPath string) error {
	return hls.ExportEmbeddedSubtitles(videoPath)
}

// TranslateExportedSubtitles exports embedded subtitles and translates them in the background
func (a *App) TranslateExportedSubtitles(fileNameOrUrl string, targetLanguage string) ([]string, error) {
	settings := a.settingsStore.Get()

	apiKey := settings.GeminiApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is required. Please set it in Settings or GEMINI_API_KEY environment variable")
	}

	if targetLanguage == "" {
		return nil, fmt.Errorf("target language is required")
	}

	return ai.TranslateForFile(a.ctx, ai.Request{
		FileNameOrURL:  fileNameOrUrl,
		TargetLanguage: targetLanguage,
		APIKey:         apiKey,
		Model:          settings.GeminiModel,
		PromptTemplate: settings.TranslatePromptTemplate,
		MaxSamples:     settings.MaxSubtitleSamples,
	})
}

// GenerateTranslationPrompt builds and returns the LLM prompt for subtitles without invoking the model
func (a *App) GenerateTranslationPrompt(fileNameOrUrl string, targetLanguage string) (string, error) {
	settings := a.settingsStore.Get()
	if targetLanguage == "" {
		return "", fmt.Errorf("target language is required")
	}
	req := ai.Request{
		FileNameOrURL:  fileNameOrUrl,
		TargetLanguage: targetLanguage,
		PromptTemplate: settings.TranslatePromptTemplate,
		MaxSamples:     settings.MaxSubtitleSamples,
	}
	return ai.GeneratePromptForFile(req)
}

// ProcessPastedTranslation accepts pasted LLM output, parses and writes translated VTT(s)
func (a *App) ProcessPastedTranslation(fileNameOrUrl string, targetLanguage string, pastedAnswer string) ([]string, error) {
	if targetLanguage == "" {
		return nil, fmt.Errorf("target language is required")
	}

	req := ai.Request{
		FileNameOrURL:  fileNameOrUrl,
		TargetLanguage: targetLanguage,
	}
	return ai.ProcessPastedForFile(a.ctx, req, pastedAnswer)
}

// GetHistory returns all history items
func (a *App) GetHistory() []HistoryItem {
	return a.historyStore.GetAll()
}

// RemoveFromHistory removes an item from history
func (a *App) RemoveFromHistory(path string) error {
	return a.historyStore.Remove(path)
}

// GetSettings returns the current settings
func (a *App) GetSettings() *Settings {
	return a.settingsStore.Get()
}

// UpdateSettings updates the settings
func (a *App) UpdateSettings(settings Settings) error {
	return a.settingsStore.Update(settings)
}

// ResetSettings resets settings to defaults
func (a *App) ResetSettings() (*Settings, error) {
	if err := a.settingsStore.Reset(); err != nil {
		return nil, err
	}
	return a.settingsStore.Get(), nil
}

// GetFFmpegInfo returns ffmpeg and ffprobe version information
func (a *App) GetFFmpegInfo(searchAgain bool) (*hls.FFmpegInfo, error) {
	return hls.GetFFmpegInfo(searchAgain)
}

// ClearHistory clears all history
func (a *App) ClearHistory() error {
	return a.historyStore.Clear()
}

// handleChromecastMessage handles messages from Chromecast
func (a *App) handleChromecastMessage(msg *cast_proto.CastMessage) {
	if msg.PayloadUtf8 == nil {
		return
	}

	messageBytes := []byte(*msg.PayloadUtf8)

	// Check message type
	var msgType struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(messageBytes, &msgType); err != nil {
		return
	}

	// Handle different message types
	switch msgType.Type {
	case "MEDIA_STATUS":
		var resp struct {
			Status []struct {
				CurrentTime float64 `json:"currentTime"`
				PlayerState string  `json:"playerState"`
				IdleReason  string  `json:"idleReason"`
			} `json:"status"`
		}

		if err := json.Unmarshal(messageBytes, &resp); err == nil && len(resp.Status) > 0 {
			status := resp.Status[0]

			a.mu.Lock()
			a.playbackState.CurrentTime = status.CurrentTime

			switch status.PlayerState {
			case "PAUSED":
				a.playbackState.Status = "PAUSED"
			case "PLAYING":
				a.playbackState.Status = "PLAYING"
			case "IDLE":
				if status.IdleReason == "FINISHED" || status.IdleReason == "INTERRUPTED" {
					a.playbackState.Status = "STOPPED"
					a.mu.Unlock()
					return
				}
			}
			a.mu.Unlock()
		}

	case "CLOSE", "LOAD_FAILED":
		a.mu.Lock()
		a.playbackState.Status = "STOPPED"
		a.mu.Unlock()
	}
	events.Emit("playback:state", a.playbackState)
}

// Helper functions

func extractDeviceName(deviceURL string) string {
	// Extract name from URL or return address
	parts := strings.Split(deviceURL, "/")
	if len(parts) > 2 {
		return parts[2]
	}
	return deviceURL
}
