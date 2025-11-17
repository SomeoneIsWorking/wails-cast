package main

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vishen/go-chromecast/application"
)

// CastToChromeCast casts media to a Chromecast device
func CastToChromeCast(ctx any, deviceAddr string, mediaURL string, duration float64) error {
	return CastToChromeCastWithSeek(ctx, deviceAddr, mediaURL, duration, 0)
}

// CastToChromeCastWithSeek casts media with duration and seek time
func CastToChromeCastWithSeek(ctx any, deviceAddr string, mediaURL string, duration float64, seekTime int) error {
	logger.Info("Attempting Chromecast cast", "device", deviceAddr, "media", mediaURL, "duration", duration, "seekTime", seekTime)

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
		return err
	}

	// Update to ensure receiver is ready
	if err := app.Update(); err != nil {
		logger.Error("Failed to update app status", "error", err)
		return err
	}

	logger.Info("Sending load command with duration and seek", "url", mediaURL, "duration", duration, "seekTime", seekTime)
	app.SetRequestTimeout(30 * time.Second)
	// Load media with HLS content type and Shaka Player
	err = app.Load(mediaURL, application.LoadOptions{
		StartTime:      seekTime,
		ContentType:    "application/x-mpegURL",
		Transcode:      false,
		Detach:         true,
		ForceDetach:    false,
		Duration:       float32(duration),
		UseShakaForHls: true, // Enable Shaka Player for HLS
	})
	if err != nil {
		logger.Error("Load failed", "error", err)
		return err
	}

	logger.Info("Load succeeded!")
	logger.Info("Media should now be playing on device", "device", host)

	return nil
}
