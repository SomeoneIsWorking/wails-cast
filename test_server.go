//go:build test
// +build test

package main

import (
	"log"
)

func main() {
	// Initialize components
	discovery := NewDeviceDiscovery()
	localIP := discovery.GetLocalIP()

	// Create media manager and server
	server := NewServer(8888, localIP)
	// Set up media

	videoPath := "-"
	subtitlePath := "-"

	server.SetCurrentMedia(videoPath)
	server.SetSubtitlePath(subtitlePath)
	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("Server started on port 8888")

	// Get duration
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		log.Printf("Warning: Could not get video duration: %v", err)
		duration = 0
	}
	log.Printf("Video duration: %.2f seconds", duration)

	// Get media URL

	log.Printf("\n========================================")
	log.Printf("Transcoding server ready!")
	log.Printf("Video: %s", videoPath)
	log.Printf("Subtitle: %s", subtitlePath)
	log.Printf("Duration: %.2f seconds", duration)
	log.Printf("========================================\n")
	log.Printf("Open the URL above in your browser or media player to test")
	log.Printf("Press Ctrl+C to exit\n")

	// Keep running
	select {}
}
