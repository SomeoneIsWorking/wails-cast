//go:build test
// +build test

package main

import (
	"log"
	"os"
)

func testCastMain() {
	// Initialize components
	discovery := NewDeviceDiscovery()

	// Discover devices
	log.Println("Starting discovery...")
	devices, err := discovery.Discover()
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}

	if len(devices) == 0 {
		log.Fatal("No devices found")
	}

	log.Printf("Found %d devices\n", len(devices))

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

	// Get device URL (first device)
	deviceURL := devices[0].URL

	// Get media URL
	mediaURL := mediaManager.GetMediaURL(videoPath)

	// Cast to device
	log.Printf("Casting %s to %s via %s (duration: %.2fs)\n", videoPath, deviceURL, mediaURL, duration)

	err = CastToChromeCast(nil, deviceURL, mediaURL, duration)
	if err != nil {
		log.Fatalf("Cast failed: %v", err)
		os.Exit(1)
	}

	log.Println("Cast successful!")

	// Keep running
	log.Println("Keeping server running... Press Ctrl+C to exit")
	select {}
}
