package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishen/go-chromecast/application"
	cast_proto "github.com/vishen/go-chromecast/cast/proto"
	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	"wails-cast/pkg/cast"
	"wails-cast/pkg/sleepinhibit"
)

// CastOptions holds options for casting
type CastOptions struct {
	SubtitlePath  string
	SubtitleTrack int // -1 for external file, >= 0 for embedded track index
}

// SubtitleTrack represents a subtitle track in a video file
type SubtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Codec    string `json:"codec"`
}

type App struct {
	ctx             context.Context
	discovery       *DeviceDiscovery
	mediaServer     *Server
	playbackState   PlaybackState
	currentSubtitle string
	chromecastApp   *cast.ChromecastApp
	sleepInhibitor  *sleepinhibit.Inhibitor
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
	CanSeek     bool    `json:"canSeek"`
}

func NewApp() *App {
	discovery := NewDeviceDiscovery()
	localIP := discovery.GetLocalIP()
	server := NewServer(8888, localIP)

	return &App{
		discovery:      discovery,
		mediaServer:    server,
		sleepInhibitor: sleepinhibit.NewInhibitor(logger),
		playbackState: PlaybackState{
			CanSeek: true,
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Start media server
	go a.mediaServer.Start()

	logger.Info("App started")
}

// startSleepInhibition prevents system sleep during streaming
func (a *App) startSleepInhibition() {
	if a.sleepInhibitor != nil {
		a.sleepInhibitor.Start()
	}
}

// stopSleepInhibition allows system sleep again
func (a *App) stopSleepInhibition() {
	if a.sleepInhibitor != nil {
		a.sleepInhibitor.Stop()
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.stopSleepInhibition()
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
	localIP := a.discovery.GetLocalIP()
	return fmt.Sprintf("http://%s:%d/media.m3u8", localIP, 8888)
}

// GetLocalIP returns the local IP address
func (a *App) GetLocalIP() string {
	return a.discovery.GetLocalIP()
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
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

// GetSubtitleTracks extracts subtitle tracks from a video file
func (a *App) GetSubtitleTracks(videoPath string) ([]SubtitleTrack, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index:stream_tags=language,title:stream=codec_name",
		"-of", "json",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse JSON output
	var result struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	tracks := make([]SubtitleTrack, 0, len(result.Streams))
	for i, stream := range result.Streams {
		track := SubtitleTrack{
			Index:    i, // Use relative subtitle index (0, 1, 2...) not absolute stream index
			Language: stream.Tags.Language,
			Title:    stream.Tags.Title,
			Codec:    stream.CodecName,
		}
		tracks = append(tracks, track)
		logger.Info("Subtitle track found", "relativeIndex", i, "absoluteStreamIndex", stream.Index, "language", stream.Tags.Language, "title", stream.Tags.Title)
	}

	return tracks, nil
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

// CastToDevice casts media to a device
func (a *App) CastToDevice(deviceURL, mediaPath string, options CastOptions) error {
	// Set media on server
	a.mediaServer.SetCurrentMedia(mediaPath)

	// Handle subtitle path - for embedded tracks, use special format
	subtitlePath := options.SubtitlePath
	if options.SubtitleTrack >= 0 {
		// Embedded subtitle track - use video file path with track index
		subtitlePath = fmt.Sprintf("%s:si=%d", mediaPath, options.SubtitleTrack)
	}
	a.mediaServer.SetSubtitlePath(subtitlePath)

	// Store current subtitle
	a.currentSubtitle = subtitlePath

	// Get duration
	duration, err := GetVideoDuration(mediaPath)
	if err != nil {
		logger.Warn("Failed to get duration", "error", err)
		duration = 0
	}

	// Update playback state
	a.mu.Lock()
	a.playbackState.IsPlaying = true
	a.playbackState.MediaPath = mediaPath
	a.playbackState.MediaName = filepath.Base(mediaPath)
	a.playbackState.DeviceURL = deviceURL
	a.playbackState.DeviceName = extractDeviceName(deviceURL)
	a.playbackState.Duration = duration
	a.mu.Unlock()

	// Get media URL
	mediaURL := a.GetMediaURL(mediaPath)

	// Cast to device - connect directly for local files
	app := application.NewApplication()
	app.SetRequestTimeout(30 * time.Second)

	// Extract host and port from deviceURL
	// deviceURL could be "http://192.168.1.21:8009", "192.168.1.21:8009", or "192.168.1.21"
	deviceAddr := deviceURL

	// Strip protocol if present
	if strings.HasPrefix(deviceAddr, "http://") {
		deviceAddr = strings.TrimPrefix(deviceAddr, "http://")
	} else if strings.HasPrefix(deviceAddr, "https://") {
		deviceAddr = strings.TrimPrefix(deviceAddr, "https://")
	}

	host := deviceAddr
	port := 8009

	// Check if port is already included
	if strings.Contains(deviceAddr, ":") {
		parts := strings.SplitN(deviceAddr, ":", 2)
		host = parts[0]
		if len(parts) == 2 {
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
			}
		}
	}

	err = app.Start(host, port)
	if err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return err
	}

	// Update to ensure receiver is ready
	if err := app.Update(); err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return err
	}

	// Load media with custom receiver
	customAppID := "4C4BFD9F"
	err = app.LoadApp(customAppID, mediaURL)
	if err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return err
	}

	// Store the chromecast app
	ccApp := &cast.ChromecastApp{
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

	// Prevent system sleep while streaming
	a.startSleepInhibition()

	logger.Info("Cast successful",
		"message", fmt.Sprintf("Casting %s to %s via %s", filepath.Base(mediaPath), deviceURL, mediaURL),
		"device", deviceURL,
		"media", mediaPath,
		"subtitle", options.SubtitlePath,
	)

	return nil
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
func (a *App) SeekTo(deviceURL, mediaPath string, seekTime float64) error {
	logger.Info("Seeking", "device", deviceURL, "media", mediaPath, "seekTime", seekTime)

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
func (a *App) UpdateSubtitleSettings(options CastOptions) error {
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
	currentTime := a.playbackState.CurrentTime
	a.mu.RUnlock()

	if ccApp != nil {
		return ccApp.App.SeekToTime(float32(currentTime))
	}

	return nil
}

// GetPlaybackState returns current playback state
func (a *App) GetPlaybackState() PlaybackState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.playbackState
}

// UpdatePlaybackState actively fetches current status from Chromecast
func (a *App) UpdatePlaybackState() (PlaybackState, error) {
	a.mu.RLock()
	ccApp := a.chromecastApp
	a.mu.RUnlock()

	if ccApp == nil {
		return a.GetPlaybackState(), nil
	}

	// Request current media status from Chromecast
	if err := ccApp.App.Update(); err != nil {
		logger.Warn("Failed to update chromecast status", "error", err)
		return a.GetPlaybackState(), err
	}

	// Get updated status
	_, media, _ := ccApp.App.Status()
	if media != nil {
		a.mu.Lock()
		a.playbackState.CurrentTime = float64(media.CurrentTime)

		switch media.PlayerState {
		case "PAUSED":
			a.playbackState.IsPaused = true
		case "PLAYING":
			a.playbackState.IsPaused = false
		case "IDLE":
			if media.IdleReason == "FINISHED" || media.IdleReason == "INTERRUPTED" {
				a.playbackState.IsPlaying = false
				a.playbackState.IsPaused = false
			}
		}
		a.mu.Unlock()
	}

	return a.GetPlaybackState(), nil
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
	a.stopSleepInhibition()
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

// ClearCache clears the HLS segment cache
func (a *App) ClearCache() error {
	if a.mediaServer != nil && a.mediaServer.hlsServer != nil {
		a.mediaServer.hlsServer.Cleanup()
		logger.Info("Cache cleared")
		return nil
	}
	return fmt.Errorf("no active session to clear")
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
					a.stopSleepInhibition()
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
		a.stopSleepInhibition()
	}
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
