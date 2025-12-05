package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vishen/go-chromecast/application"
	cast_proto "github.com/vishen/go-chromecast/cast/proto"
	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wails-cast/pkg/ai"
	localcast "wails-cast/pkg/cast"
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
	ctx           context.Context
	App           *application.Application
	discovery     *DeviceDiscovery
	mediaServer   *Server
	localIp       string
	castManager   *localcast.CastManager
	playbackState PlaybackState
	historyStore  *HistoryStore
	settingsStore *SettingsStore
	mu            sync.RWMutex
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
	server := NewServer(localIP, 8888)
	castManager := localcast.NewCastManager(localIP, 8888)
	historyStore := NewHistoryStore()
	settingsStore := NewSettingsStore()

	return &App{
		discovery:     discovery,
		mediaServer:   server,
		localIp:       localIP,
		castManager:   castManager,
		playbackState: PlaybackState{},
		historyStore:  historyStore,
		settingsStore: settingsStore,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Set context for settings store
	a.settingsStore.SetContext(ctx)
	// Set context for history store to enable events
	a.historyStore.SetContext(ctx)
	// Set context for media server to enable error events
	a.mediaServer.SetContext(ctx)

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
	err := a.discovery.DiscoverStream(func(d Device) {
		wails_runtime.EventsEmit(a.ctx, "device:found", d)
	}, func() {
		wails_runtime.EventsEmit(a.ctx, "discovery:complete")
	})
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
		trackInfo, err = a.castManager.GetRemoteTrackInfo(fileNameOrUrl)
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
			Label: fmt.Sprintf("Embedded: %s", sub.Language),
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
func (a *App) CastToDevice(deviceIp string, fileNameOrUrl string, options options.CastOptions) (*PlaybackState, error) {
	// Determine if input is a local file or remote URL
	isRemote := strings.HasPrefix(fileNameOrUrl, "http://") || strings.HasPrefix(fileNameOrUrl, "https://")

	var mediaPath string
	var duration float64
	var err error

	host := deviceIp
	port := 8009

	if isRemote {
		mediaPath = fileNameOrUrl
		// Use CastManager to prepare remote stream
		logger.Info("Preparing remote stream", "url", mediaPath)
		handler, err := a.castManager.CreateRemoteHandler(mediaPath, options)
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
		handler := stream.NewLocalHandler(mediaPath, options, a.localIp)
		a.mediaServer.SetHandler(handler)

		// Set subtitle path (for legacy/compatibility, though options has it)
		a.mediaServer.SetSubtitlePath(options.Subtitle.Path)
	}

	if deviceIp == "local" {
		// Just host the stream without casting
		logger.Info("Hosting stream without casting", "url", a.GetMediaURL(mediaPath))
		return &a.playbackState, nil
	}

	// Update playback state
	a.mu.Lock()
	a.playbackState.MediaPath = mediaPath
	a.playbackState.MediaName = filepath.Base(mediaPath)
	a.playbackState.DeviceURL = deviceIp
	a.playbackState.DeviceName = extractDeviceName(deviceIp)
	a.playbackState.Duration = duration
	a.mu.Unlock()

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
	if err := a.historyStore.Add(mediaPath, extractDeviceName(deviceIp)); err != nil {
		logger.Warn("Failed to add to history", "error", err)
	}

	return &a.playbackState, nil
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
	return nil
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
	return a.stopPlayback(true)
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
func (a *App) LogInfo(message string) {
	logger.Info(message)
}

// LogWarn logs a warning message from frontend
func (a *App) LogWarn(message string) {
	logger.Warn(message)
}

// LogError logs an error message from frontend
func (a *App) LogError(message string) {
	logger.Error(message)
}

// ExportEmbeddedSubtitles exports all embedded subtitle tracks to WebVTT files
func (a *App) ExportEmbeddedSubtitles(videoPath string) error {
	if strings.HasPrefix(videoPath, "http://") || strings.HasPrefix(videoPath, "https://") {
		return fmt.Errorf("cannot export subtitles from remote URLs")
	}

	trackInfo, err := hls.GetMediaTrackInfo(videoPath)
	if err != nil {
		return fmt.Errorf("failed to get track info: %w", err)
	}

	if len(trackInfo.SubtitleTracks) == 0 {
		return fmt.Errorf("no embedded subtitles found")
	}

	baseDir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	subtitleDir := filepath.Join(baseDir, baseName)

	// Create subtitle directory
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return fmt.Errorf("failed to create subtitle directory: %w", err)
	}

	for _, sub := range trackInfo.SubtitleTracks {
		lang := sub.Language
		if lang == "" {
			lang = fmt.Sprintf("track%d", sub.Index)
		}

		outputFile := filepath.Join(subtitleDir, fmt.Sprintf("%s.vtt", lang))

		// Use ffmpeg to extract subtitle
		args := []string{
			"-i", videoPath,
			"-map", fmt.Sprintf("0:s:%d", sub.Index),
			"-f", "webvtt",
			"-y", // Overwrite output file
			outputFile,
		}

		if err := hls.RunFFmpeg(args...); err != nil {
			logger.Warn("Failed to export subtitle", "index", sub.Index, "language", lang, "error", err)
			continue
		}

		logger.Info("Exported subtitle", "file", outputFile)
	}

	return nil
}

// TranslateExportedSubtitles exports embedded subtitles and translates them in the background
func (a *App) TranslateExportedSubtitles(videoPath string, targetLanguage string) error {
	// Get settings for API key and model
	settings := a.settingsStore.Get()

	// Use settings API key, fallback to environment variable
	apiKey := settings.GeminiApiKey

	if apiKey == "" {
		return fmt.Errorf("Gemini API key is required. Please set it in Settings or GEMINI_API_KEY environment variable")
	}

	// Use target language from settings if not provided
	if targetLanguage == "" {
		targetLanguage = settings.DefaultTranslationLanguage
	}
	if targetLanguage == "" {
		return fmt.Errorf("target language is required")
	}

	// Determine subtitle directory
	baseDir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	subtitleDir := filepath.Join(baseDir, baseName)

	// Check if subtitle directory exists, if not export the subtitles
	if _, err := os.Stat(subtitleDir); os.IsNotExist(err) {
		logger.Info("Subtitle directory doesn't exist, exporting subtitles", "dir", subtitleDir)
		if err := a.ExportEmbeddedSubtitles(videoPath); err != nil {
			return fmt.Errorf("failed to export subtitles: %w", err)
		}
	} else {
		logger.Info("Using existing exported subtitles", "dir", subtitleDir)
	}

	// Run translation in background
	go func() {
		// Create translator with model from settings
		translator, err := ai.NewTranslator(apiKey, settings.GeminiModel)
		if err != nil {
			wails_runtime.EventsEmit(a.ctx, "translation:error", fmt.Sprintf("Failed to create translator: %v", err))
			return
		}
		defer translator.Close()

		// Translate all exported subtitles
		logger.Info("Translating exported subtitles", "directory", subtitleDir, "target", targetLanguage, "model", settings.GeminiModel)
		translatedFiles, err := translator.TranslateEmbeddedSubtitles(a.ctx, subtitleDir, targetLanguage, func(chunk string) {
			// Stream translation progress to frontend
			wails_runtime.EventsEmit(a.ctx, "translation:stream", chunk)
		})
		if err != nil {
			wails_runtime.EventsEmit(a.ctx, "translation:error", fmt.Sprintf("Translation failed: %v", err))
			return
		}

		logger.Info("Translation completed", "files", len(translatedFiles))
		// Emit completion event with translated file paths
		wails_runtime.EventsEmit(a.ctx, "translation:complete", translatedFiles)
	}()

	// Return immediately - translation started
	return nil
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
	wails_runtime.EventsEmit(a.ctx, "playback:state", a.playbackState)
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
