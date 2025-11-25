package cast

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SelectTracksInteractive parses the master playlist and allows the user to select tracks
func SelectTracksInteractive(manifest string) (string, error) {
	lines := strings.Split(manifest, "\n")
	var videoTracks []string
	var audioTracks []string
	var otherLines []string

	// Simple parsing
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			videoTracks = append(videoTracks, line+"\n"+lines[i+1])
			i++ // Skip next line as it is the URL
		} else if strings.HasPrefix(line, "#EXT-X-MEDIA:TYPE=AUDIO") {
			audioTracks = append(audioTracks, line)
		} else {
			otherLines = append(otherLines, line)
		}
	}

	if len(videoTracks) == 0 {
		return manifest, nil // No tracks to select
	}

	fmt.Println("\n🎥 Available Video Tracks:")
	for i, track := range videoTracks {
		// Extract resolution/bandwidth for display
		info := ""
		if strings.Contains(track, "RESOLUTION=") {
			start := strings.Index(track, "RESOLUTION=") + 11
			end := strings.Index(track[start:], ",")
			if end == -1 {
				end = len(track[start:])
			}
			info += "Res: " + track[start:start+end] + " "
		}
		if strings.Contains(track, "BANDWIDTH=") {
			start := strings.Index(track, "BANDWIDTH=") + 10
			end := strings.Index(track[start:], ",")
			if end == -1 {
				end = len(track[start:])
			}
			info += "Bw: " + track[start:start+end]
		}
		fmt.Printf("[%d] %s\n", i+1, info)
	}

	fmt.Print("Select video track (default 1): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	videoIdx := 0
	if input != "" {
		idx, err := strconv.Atoi(input)
		if err == nil && idx > 0 && idx <= len(videoTracks) {
			videoIdx = idx - 1
		}
	}

	selectedVideo := videoTracks[videoIdx]

	// Audio selection
	selectedAudio := ""
	if len(audioTracks) > 0 {
		fmt.Println("\n🔊 Available Audio Tracks:")
		for i, track := range audioTracks {
			// Extract name/lang
			info := ""
			if strings.Contains(track, "NAME=\"") {
				start := strings.Index(track, "NAME=\"") + 6
				end := strings.Index(track[start:], "\"")
				info += "Name: " + track[start:start+end] + " "
			}
			if strings.Contains(track, "LANGUAGE=\"") {
				start := strings.Index(track, "LANGUAGE=\"") + 10
				end := strings.Index(track[start:], "\"")
				info += "Lang: " + track[start:start+end]
			}
			fmt.Printf("[%d] %s\n", i+1, info)
		}

		fmt.Print("Select audio track (default 1): ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		audioIdx := 0
		if input != "" {
			idx, err := strconv.Atoi(input)
			if err == nil && idx > 0 && idx <= len(audioTracks) {
				audioIdx = idx - 1
			}
		}
		selectedAudio = audioTracks[audioIdx]
	}

	// Reconstruct manifest
	var newManifest strings.Builder
	newManifest.WriteString("#EXTM3U\n")
	newManifest.WriteString("#EXT-X-VERSION:3\n")

	// Add selected audio if any
	if selectedAudio != "" {
		newManifest.WriteString(selectedAudio + "\n")
	}

	// Add selected video
	newManifest.WriteString(selectedVideo + "\n")

	return newManifest.String(), nil
}
