package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"wails-cast/pkg/cast"
)

// CastRemoteURL casts a remote video URL to a Chromecast device
func (a *App) CastRemoteURL(videoURL, deviceURL string) error {
	// Extract host and port from deviceURL
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

	// Get local IP
	localIP, err := getLocalIPAddr()
	if err != nil {
		return fmt.Errorf("failed to get local IP: %w", err)
	}

	// Create cast manager
	manager := cast.NewCastManager(localIP, 8889)

	// Start casting in background
	go func() {
		logger.Info("Starting remote URL cast", "url", videoURL, "device", deviceAddr)
		if err := manager.StartCasting(videoURL, host, port); err != nil {
			logger.Error("Failed to cast remote URL", "error", err)
		}
	}()

	return nil
}

// getLocalIPAddr gets the local IP address
func getLocalIPAddr() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no local IP found")
}
