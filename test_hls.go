//go:build test
// +build test

package main

import (
	"fmt"
	"time"
)

func main() {
	// Setup
	discovery := NewDeviceDiscovery()
	localIP := discovery.GetLocalIP()
	server := NewServer(8888, localIP)

	// Start server
	go server.Start()
	time.Sleep(1 * time.Second) // Set media
	videoPath := "-"
	subtitlePath := "-"

	server.SetCurrentMedia(videoPath)
	server.SetSubtitlePath(subtitlePath)
	server.SetSeekTime(0)

	// Get duration
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		fmt.Printf("Failed to get duration: %v\n", err)
		duration = 0
	}

	fmt.Printf("Duration: %.2f seconds\n", duration)

	// Get media URL
	mediaURL := fmt.Sprintf("http://%s:8888/media.m3u8", localIP)

	fmt.Printf("Media URL: %s\n", mediaURL)
	fmt.Printf("Casting to 192.168.1.21:8009...\n")

	// Cast
	deviceURL := "http://192.168.1.21:8009"
	err = CastToChromeCastWithSeek(nil, deviceURL, mediaURL, duration, 0)
	if err != nil {
		fmt.Printf("Cast failed: %v\n", err)
		return
	}

	fmt.Printf("Cast successful! Waiting for playback...\n")

	// Keep running
	select {}
}
