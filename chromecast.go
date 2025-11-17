package main

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vishen/go-chromecast/application"
)

// ChromecastApp wraps the chromecast application
type ChromecastApp struct {
	App  *application.Application
	Host string
	Port int
}

// CastToChromeCastWithSeek casts media with duration and seek time
func CastToChromeCast(ctx any, deviceAddr string, mediaURL string, duration float64) (*ChromecastApp, error) {
	logger.Info("Attempting Chromecast cast", "device", deviceAddr, "media", mediaURL, "duration", duration)

	// Extract host from device URL if it's a full URL
	host := deviceAddr
	port := 8009

	// If deviceAddr looks like a URL, parse it
	if strings.HasPrefix(deviceAddr, "http") {
		parsedURL, err := url.Parse(deviceAddr)
		if err == nil {
			hostname := parsedURL.Hostname()
			if hostname != "" {
				host = hostname
			}
		}
	}

	// Try to split host and port if provided
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if portNum, err := strconv.Atoi(p); err == nil {
			port = portNum
		}
	}

	logger.Info("Connecting to device", "host", host, "port", port)

	// Create application
	app := application.NewApplication()

	// Start the application (this is the correct way)
	logger.Info("Starting application on device")
	err := app.Start(host, port)
	if err != nil {
		logger.Error("Failed to start app", "error", err)
		return nil, err
	}

	// Update to ensure receiver is ready
	if err := app.Update(); err != nil {
		logger.Error("Failed to update app status", "error", err)
		return nil, err
	}

	logger.Info("Sending load command with duration", "url", mediaURL, "duration", duration)
	app.SetRequestTimeout(30 * time.Second)
	// Load media with HLS content type and Shaka Player
	err = app.Load(mediaURL, application.LoadOptions{
		StartTime:      0,
		ContentType:    "application/x-mpegURL",
		Transcode:      false,
		Detach:         true,
		ForceDetach:    false,
		Duration:       float32(duration),
		UseShakaForHls: true, // Enable Shaka Player for HLS
	})
	if err != nil {
		logger.Error("Load failed", "error", err)
		return nil, err
	}

	logger.Info("Load succeeded!")
	logger.Info("Media should now be playing on device", "device", host)

	return &ChromecastApp{
		App:  app,
		Host: host,
		Port: port,
	}, nil
}
