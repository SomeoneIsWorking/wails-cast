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
	"github.com/vishen/go-chromecast/cast"
	cast_proto "github.com/vishen/go-chromecast/cast/proto"
	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	localcast "wails-cast/pkg/cast"
	"wails-cast/pkg/hls"
	_inhibitor "wails-cast/pkg/inhibitor"
	_logger "wails-cast/pkg/logger"
	"wails-cast/pkg/mediainfo"
	"wails-cast/pkg/stream"
)

var logger = _logger.Logger
var inhibitor = _inhibitor.InhibitorInstance

// CastOptions holds options for casting
type CastOptions struct {
	SubtitlePath  string
	SubtitleTrack int  // -1 for external file, >= 0 for embedded track index
	VideoTrack    int  // -1 for default
	AudioTrack    int  // -1 for default
	BurnIn        bool // true to burn subtitles into video
	CRF           int  // quality setting (e.g., 23 for high quality)
	Debug         bool // true to enable debug mode
}

type SubtitleOptions struct {
	SubtitlePath  string
	SubtitleTrack int // -1 for external file, >= 0 for embedded track index
	BurnIn        bool
}

type QualityOption struct {
	Label string
	CRF   int
	Key   string
}

type App struct {
	ctx             context.Context
	discovery       *DeviceDiscovery
	mediaServer     *Server
	localIp         string
	castManager     *localcast.CastManager
	playbackState   PlaybackState
	currentSubtitle string
	chromecastApp   *localcast.ChromecastApp
	mu              sync.RWMutex
}

type PlaybackState struct {
	IsPlaying   bool    `json:"isPlaying"`
	IsPaused    bool    `json:"isPaused"`
	MediaPath   string  `json:"mediaPath"`
	MediaName   string  `json:"mediaName"`
	DeviceURL   string  `json:"deviceUrl"`
	DeviceName  string  `json:"deviceName"`
	CurrentTime float64 `json:"currentTime"`
	Duration    float64 `json:"duration"`
}

func NewApp() *App {
	discovery := NewDeviceDiscovery()
	localIP := discovery.GetLocalIP()
	server := NewServer(localIP, 8888)
	castManager := localcast.NewCastManager(localIP, 8888)

	return &App{
		discovery:     discovery,
		mediaServer:   server,
		localIp:       localIP,
		castManager:   castManager,
		playbackState: PlaybackState{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Start media server
	go a.mediaServer.Start()

	logger.Info("App started")
}

func (a *App) shutdown(ctx context.Context) {
	inhibitor.Stop()
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

func (a *App) GetQualityOptions() []QualityOption {
	return []QualityOption{
		{Label: "Low (CRF 35)", CRF: 35, Key: "low"},
		{Label: "Medium (CRF 28)", CRF: 28, Key: "medium"},
		{Label: "High (CRF 23)", CRF: 23, Key: "high"},
		{Label: "Original (Best Quality)", CRF: 18, Key: "original"},
	}
}

func (a *App) GetDefaultQuality() string {
	return "medium"
}

// GetSubtitleURL returns the URL for subtitle file (for Shaka player)
func (a *App) GetSubtitleURL(subtitlePath string) string {
	if subtitlePath == "" {
		return ""
	}
	return "/subtitle.vtt"
}

// GetVideoDuration returns the duration of a video file in seconds
func GetVideoDuration(videoPath string) (float64, error) {
	return hls.GetVideoDuration(videoPath)
}

// GetSubtitleTracks extracts subtitle tracks from a video file
func (a *App) GetSubtitleTracks(videoPath string) ([]mediainfo.SubtitleTrack, error) {
	return mediainfo.GetSubtitleTracks(videoPath)
}

// OpenSubtitleDialog opens a file picker dialog for subtitle files
func (a *App) OpenSubtitleDialog() (string, error) {
	return wails_runtime.OpenFileDialog(a.ctx, wails_runtime.OpenDialogOptions{
		Title: "Select Subtitle File",
		Filters: []wails_runtime.FileFilter{
			{
				DisplayName: "Subtitle Files",
				Pattern:     "*.srt;*.vtt;*.ass;*.ssa",
			},
		},
	})
}

// GetMediaTrackInfo gets all track information for a media file
func (a *App) GetMediaTrackInfo(mediaPath string) (*mediainfo.MediaTrackInfo, error) {
	return mediainfo.GetMediaTrackInfo(mediaPath)
}

// GetRemoteTrackInfo gets track information from a remote HLS stream
func (a *App) GetRemoteTrackInfo(videoURL string) (*mediainfo.MediaTrackInfo, error) {
	return a.castManager.GetRemoteTrackInfo(videoURL)
}

// CastToDevice casts media (local file or remote URL) to a device
func (a *App) CastToDevice(deviceIp string, fileNameOrUrl string, options CastOptions) (*PlaybackState, error) {
	// Determine if input is a local file or remote URL
	isRemote := strings.HasPrefix(fileNameOrUrl, "http://") || strings.HasPrefix(fileNameOrUrl, "https://")

	var mediaPath string
	var duration float64
	var err error

	host := deviceIp
	port := 8009

	streamOpts := stream.StreamOptions{
		SubtitlePath:  options.SubtitlePath,
		SubtitleTrack: options.SubtitleTrack,
		VideoTrack:    options.VideoTrack,
		AudioTrack:    options.AudioTrack,
		BurnIn:        options.BurnIn,
		CRF:           options.CRF,
	}

	if isRemote {
		mediaPath = fileNameOrUrl
		// Use CastManager to prepare remote stream
		logger.Info("Preparing remote stream", "url", mediaPath)
		handler, err := a.castManager.CreateRemoteHandler(mediaPath, streamOpts)
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

		// Handle subtitle path - for embedded tracks, use video file path with track index
		// This logic is also in LocalHandler/ffmpeg but we set it here for reference
		subtitlePath := options.SubtitlePath
		if options.SubtitleTrack >= 0 {
			subtitlePath = fmt.Sprintf("%s:si=%d", mediaPath, options.SubtitleTrack)
		}

		// Create local handler
		handler := stream.NewLocalHandler(mediaPath, streamOpts, a.localIp)
		a.mediaServer.SetHandler(handler)

		// Set subtitle path (for legacy/compatibility, though options has it)
		a.mediaServer.SetSubtitlePath(subtitlePath)
		a.currentSubtitle = subtitlePath
	}

	// Update playback state
	a.mu.Lock()
	a.playbackState.IsPlaying = true
	a.playbackState.MediaPath = mediaPath
	a.playbackState.MediaName = filepath.Base(mediaPath)
	a.playbackState.DeviceURL = deviceIp
	a.playbackState.DeviceName = extractDeviceName(deviceIp)
	a.playbackState.Duration = duration
	a.mu.Unlock()

	mediaURL := a.GetMediaURL(mediaPath)

	// Cast to device
	app := application.NewApplication()
	app.SetRequestTimeout(30 * time.Second)
	app.SetDebug(true)

	err = app.Start(host, port)
	if err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return nil, err
	}

	// Update to ensure receiver is ready
	if err := app.Update(); err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return nil, err
	}

	// Load media with custom receiver
	customAppID := "4C4BFD9F"

	// Ensure we're running the custom app
	if err := app.EnsureIsAppID(customAppID); err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return nil, fmt.Errorf("failed to launch custom receiver app: %w", err)
	}

	url := mediaURL + "?cachebust=" + time.Now().Format("20060102150405")
	if options.Debug {
		url += "&debug=true"
	}
	// Send load command without waiting (LoadApp blocks with MediaWait)
	// We just want to start playback and return immediately
	err = app.SendMediaRecv(&cast.LoadMediaCommand{
		PayloadHeader: cast.LoadHeader,
		CurrentTime:   0,
		Autoplay:      true,
		Media: cast.MediaItem{
			ContentId:  url,
			StreamType: "BUFFERED",
		},
	})

	if err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return nil, err
	}

	// Store the chromecast app
	ccApp := &localcast.ChromecastApp{
		App:  app,
		Host: host,
		Port: port,
	}

	// Store the chromecast app
	a.mu.Lock()
	a.chromecastApp = ccApp
	a.mu.Unlock()

	// Register message handler for real-time updates
	ccApp.App.AddMessageFunc(a.handleChromecastMessage)

	// Do initial update to populate media state (needed for seek/pause to work)
	if err := ccApp.App.Update(); err != nil {
		logger.Warn("Failed initial chromecast status update", "error", err)
	}

	logger.Info("Cast successful",
		"message", fmt.Sprintf("Casting %s to %s via %s", filepath.Base(mediaPath), deviceIp, mediaURL),
		"device", deviceIp,
		"media", mediaPath,
		"subtitle", options.SubtitlePath,
	)

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

// FindSubtitleFile finds a subtitle file for a video
func (a *App) FindSubtitleFile(videoPath string) string {
	baseName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	exts := []string{".srt", ".vtt", ".ass", ".ssa"}

	for _, ext := range exts {
		subPath := baseName + ext
		if fileExists(subPath) {
			logger.Info("Found subtitle file", "path", subPath)
			return subPath
		}
	}

	return ""
}

// SeekTo seeks to a specific time
func (a *App) SeekTo(seekTime float64) error {

	a.mu.Lock()
	ccApp := a.chromecastApp
	a.mu.Unlock()

	if ccApp == nil {
		return fmt.Errorf("no active chromecast connection")
	}

	// Send seek command to Chromecast
	err := ccApp.App.SeekToTime(float32(seekTime))
	if err != nil {
		logger.Error("Seek failed", "error", err)
		return err
	}

	// Update playback state
	a.mu.Lock()
	a.playbackState.CurrentTime = seekTime
	a.mu.Unlock()

	logger.Info("Seek successful", "time", seekTime)
	return nil
}

// UpdateSubtitleSettings updates subtitle settings for current media without recasting
func (a *App) UpdateSubtitleSettings(options SubtitleOptions) error {
	subtitlePath := options.SubtitlePath
	if options.SubtitleTrack >= 0 {
		subtitlePath = fmt.Sprintf("%s:si=%d", a.playbackState.MediaPath, options.SubtitleTrack)
	}

	// Update subtitle path on server (clears cache)
	a.mediaServer.SetSubtitlePath(subtitlePath)
	a.currentSubtitle = subtitlePath

	// Seek to current position to reload with new subtitles
	a.mu.RLock()
	ccApp := a.chromecastApp
	a.mu.RUnlock()
	ccApp.App.Update()

	currentTime := a.playbackState.CurrentTime
	if ccApp != nil {
		return ccApp.App.SeekToTime(float32(currentTime))
	}

	return nil
}

func (a *App) ClearCache() error {
	return nil
}

// Pause pauses current playback
func (a *App) Pause() error {
	a.mu.Lock()
	ccApp := a.chromecastApp
	a.mu.Unlock()

	if ccApp == nil {
		return fmt.Errorf("no active chromecast connection")
	}

	err := ccApp.App.Pause()
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.playbackState.IsPaused = true
	a.mu.Unlock()

	return nil
}

// Unpause resumes current playback
func (a *App) Unpause() error {
	a.mu.Lock()
	ccApp := a.chromecastApp
	a.mu.Unlock()

	if ccApp == nil {
		return fmt.Errorf("no active chromecast connection")
	}

	err := ccApp.App.Unpause()
	if err != nil {
		return err
	}

	a.mu.Lock()
	a.playbackState.IsPaused = false
	a.mu.Unlock()

	return nil
}

// StopPlayback stops current playback
func (a *App) StopPlayback() {
	a.mu.Lock()
	if a.chromecastApp != nil {
		a.chromecastApp.App.Close(true)
		a.chromecastApp = nil
	}
	a.playbackState.IsPlaying = false
	a.playbackState.IsPaused = false
	a.playbackState.CurrentTime = 0
	a.mu.Unlock()

	// Allow system sleep again
	inhibitor.Stop()
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

// OpenDirectoryDialog opens a directory picker dialog
func (a *App) OpenDirectoryDialog() (string, error) {
	return wails_runtime.OpenDirectoryDialog(a.ctx, wails_runtime.OpenDialogOptions{
		Title: "Select Directory",
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
				a.playbackState.IsPaused = true
			case "PLAYING":
				a.playbackState.IsPaused = false
			case "IDLE":
				if status.IdleReason == "FINISHED" || status.IdleReason == "INTERRUPTED" {
					a.playbackState.IsPlaying = false
					a.playbackState.IsPaused = false
					a.mu.Unlock()
					// Allow system sleep when playback finishes
					inhibitor.Stop()
					return
				}
			}
			a.mu.Unlock()
		}

	case "CLOSE", "LOAD_FAILED":
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.playbackState.IsPaused = false
		a.mu.Unlock()
		// Allow system sleep when playback closes/fails
		inhibitor.Stop()
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
