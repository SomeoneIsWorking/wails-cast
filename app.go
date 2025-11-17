package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// CastOptions holds options for casting
type CastOptions struct {
	SubtitlePath string
}

type App struct {
	ctx             context.Context
	discovery       *DeviceDiscovery
	mediaServer     *Server
	playbackState   PlaybackState
	currentSubtitle string
	chromecastApp   *ChromecastApp
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
		discovery:   discovery,
		mediaServer: server,
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

func (a *App) shutdown(ctx context.Context) {
	a.mediaServer.Stop()
}

// DiscoverDevices discovers Chromecast devices on the network
func (a *App) DiscoverDevices() []Device {
	devices, err := a.discovery.Discover()
	if err != nil {
		logger.Error("Device discovery failed", "error", err)
		return []Device{}
	}
	return devices
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

// CastToDevice casts media to a device
func (a *App) CastToDevice(deviceURL, mediaPath string, options CastOptions) error {
	// Set media on server
	a.mediaServer.SetCurrentMedia(mediaPath)
	a.mediaServer.SetSubtitlePath(options.SubtitlePath)

	// Store current subtitle
	a.currentSubtitle = options.SubtitlePath

	// Get duration
	duration, err := GetVideoDuration(mediaPath)
	if err != nil {
		logger.Warn("Failed to get duration", "error", err)
		duration = 0
	}

	// Get seek time from server
	a.mu.RLock()
	seekTime := a.playbackState.CurrentTime
	a.mu.RUnlock()

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

	// Cast to device and get chromecast app
	ccApp, err := CastToChromeCastWithSeek(a.ctx, deviceURL, mediaURL, duration, int(seekTime))
	if err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return err
	}

	// Store the chromecast app
	a.mu.Lock()
	a.chromecastApp = ccApp
	a.mu.Unlock()

	// Start polling status
	go a.pollChromecastStatus()

	logger.Info("Cast successful",
		"message", fmt.Sprintf("Casting %s to %s via %s", filepath.Base(mediaPath), deviceURL, mediaURL),
		"device", deviceURL,
		"media", mediaPath,
		"subtitle", options.SubtitlePath,
		"seekTime", seekTime,
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

// GetPlaybackState returns current playback state
func (a *App) GetPlaybackState() PlaybackState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.playbackState
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
}

// OpenFileDialog opens a file picker dialog
func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Video File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Video Files",
				Pattern:     "*.mp4;*.mkv;*.avi;*.mov;*.flv;*.webm;*.m4v",
			},
		},
	})
}

// OpenDirectoryDialog opens a directory picker dialog
func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
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

// pollChromecastStatus polls Chromecast for status updates
func (a *App) pollChromecastStatus() {
	for {
		a.mu.RLock()
		ccApp := a.chromecastApp
		isPlaying := a.playbackState.IsPlaying
		a.mu.RUnlock()

		if ccApp == nil || !isPlaying {
			return
		}

		// Update status from Chromecast
		err := ccApp.App.Update()
		if err != nil {
			logger.Warn("Failed to update Chromecast status", "error", err)
			continue
		}

		// Get current media status
		_, media, _ := ccApp.App.Status()
		if media != nil {
			a.mu.Lock()
			a.playbackState.CurrentTime = float64(media.CurrentTime)
			// Update pause state based on PlayerState
			if media.PlayerState == "PAUSED" {
				a.playbackState.IsPaused = true
			} else if media.PlayerState == "PLAYING" {
				a.playbackState.IsPaused = false
			} else if media.PlayerState == "IDLE" {
				a.playbackState.IsPlaying = false
				a.playbackState.IsPaused = false
			}
			a.mu.Unlock()
		}

		// Poll every 2 seconds
		time.Sleep(2 * time.Second)
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
