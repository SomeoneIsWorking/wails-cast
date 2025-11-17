//go:build test
// +build test

package main

import (
	"log"
)

func testServerMain() {
	// Initialize components
	discovery := NewDeviceDiscovery()

	// Create media manager and server
	mediaManager := NewMediaManager(discovery, 8888)
	server := NewServer(8888, mediaManager)

	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Println("Server started on port 8888")

	// Set up media
	videoPath := "-"
	subtitlePath := "-"

	// Get duration
	duration, err := GetVideoDuration(videoPath)
	if err != nil {
		log.Printf("Warning: Could not get video duration: %v", err)
		duration = 0
	}
	log.Printf("Video duration: %.2f seconds", duration)

	server.SetCurrentMedia(videoPath)
	server.SetSubtitlePath(subtitlePath)

	// Get media URL
	mediaURL := mediaManager.GetMediaURL(videoPath)

	log.Printf("\n========================================")
	log.Printf("Transcoding server ready!")
	log.Printf("Video: %s", videoPath)
	log.Printf("Subtitle: %s", subtitlePath)
	log.Printf("Duration: %.2f seconds", duration)
	log.Printf("\nStream URL: %s", mediaURL)
	log.Printf("========================================\n")
	log.Printf("Open the URL above in your browser or media player to test")
	log.Printf("Press Ctrl+C to exit\n")

	// Keep running
	select {}
}
