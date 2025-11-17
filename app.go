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
	mu              sync.RWMutex
}

type PlaybackState struct {
	IsPlaying   bool    `json:"isPlaying"`
	MediaPath   string  `json:"mediaPath"`
	MediaName   string  `json:"mediaName"`
	DeviceURL   string  `json:"deviceUrl"`
	DeviceName  string  `json:"deviceName"`
	CurrentTime int     `json:"currentTime"`
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

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	// Simplified - can be implemented with exec.Command
	return nil
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

	// Cast to device
	err = CastToChromeCastWithSeek(a.ctx, deviceURL, mediaURL, duration, seekTime)
	if err != nil {
		a.mu.Lock()
		a.playbackState.IsPlaying = false
		a.mu.Unlock()
		return err
	}

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
func (a *App) SeekTo(deviceURL, mediaPath string, seekTime int) error {
	logger.Info("Seeking", "device", deviceURL, "media", mediaPath, "seekTime", seekTime)

	// Update seek time in server
	a.mediaServer.SetSeekTime(seekTime)

	// Update playback state
	a.mu.Lock()
	a.playbackState.CurrentTime = seekTime
	a.mu.Unlock()

	// Re-cast with new position
	options := CastOptions{SubtitlePath: a.currentSubtitle}
	return a.CastToDevice(deviceURL, mediaPath, options)
}

// GetPlaybackState returns current playback state
func (a *App) GetPlaybackState() PlaybackState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.playbackState
}

// StopPlayback stops current playback
func (a *App) StopPlayback() {
	a.mu.Lock()
	a.playbackState.IsPlaying = false
	a.playbackState.CurrentTime = 0
	a.mu.Unlock()
}

// PlayLocally opens media in default browser
func (a *App) PlayLocally(mediaPath string) error {
	a.mediaServer.SetCurrentMedia(mediaPath)
	mediaURL := a.GetMediaURL(mediaPath)
	return openBrowser(mediaURL)
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
