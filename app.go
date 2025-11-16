package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx             context.Context
	deviceDiscovery *DeviceDiscovery
	mediaManager    *MediaManager
	mediaServer     *Server
	serverPort      int
}

// NewApp creates a new App application struct
func NewApp() *App {
	discovery := NewDeviceDiscovery()
	mediaManager := NewMediaManager(discovery, 8888)
	mediaServer := NewServer(8888, mediaManager)

	return &App{
		deviceDiscovery: discovery,
		mediaManager:    mediaManager,
		mediaServer:     mediaServer,
		serverPort:      8888,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mediaServer.Start()
}

// LogInfo logs an info message from the frontend
func (a *App) LogInfo(message string, args ...interface{}) {
	fmt.Printf("[INFO] %s\n", fmt.Sprintf(message, args...))
}

// LogWarn logs a warning message from the frontend
func (a *App) LogWarn(message string, args ...interface{}) {
	fmt.Printf("[WARN] %s\n", fmt.Sprintf(message, args...))
}

// LogError logs an error message from the frontend
func (a *App) LogError(message string, args ...interface{}) {
	fmt.Printf("[ERROR] %s\n", fmt.Sprintf(message, args...))
}

// OpenFileDialog opens a file picker dialog
func (a *App) OpenFileDialog() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Media File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Video Files",
				Pattern:     "*.mp4;*.mkv;*.webm;*.avi;*.mov;*.flv;*.m4v",
			},
			{
				DisplayName: "All Files",
				Pattern:     "*",
			},
		},
	})
	return file, err
}

// OpenDirectoryDialog opens a directory picker dialog
func (a *App) OpenDirectoryDialog() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Media Folder",
	})
	return dir, err
}

// DiscoverDevices discovers available cast devices on the network
func (a *App) DiscoverDevices() []Device {
	devices, err := a.deviceDiscovery.Discover()
	if err != nil {
		println("Discovery error:", err.Error())
		return []Device{}
	}
	return devices
}

// GetLocalIP returns the local IP address
func (a *App) GetLocalIP() string {
	return a.deviceDiscovery.GetLocalIP()
}

// GetMediaURL returns the URL for a media file to be cast
func (a *App) GetMediaURL(filePath string) string {
	return a.mediaManager.GetMediaURL(filePath)
}

// CastToDevice casts media to a device
func (a *App) CastToDevice(deviceURL, mediaPath string) error {
	success, message, err := a.mediaManager.CastToDevice(deviceURL, mediaPath)
	if err != nil {
		fmt.Printf("[ERROR] Cast error: %s\n", err.Error())
		return err
	}
	if !success {
		fmt.Printf("[ERROR] Cast failed: %s\n", message)
		return fmt.Errorf(message)
	}
	fmt.Printf("[INFO] Cast successful: %s\n", message)
	return nil
}

// GetMediaFiles returns media files from a directory
func (a *App) GetMediaFiles(dirPath string) []string {
	files, err := a.mediaManager.GetMediaFiles(dirPath)
	if err != nil {
		println("Error reading directory:", err.Error())
		return []string{}
	}
	return files
}
