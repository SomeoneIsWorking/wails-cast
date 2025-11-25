package hlsproxy

import (
	"fmt"
	"net/url"
	"strings"
)

// PlaylistType represents the type of HLS playlist
type PlaylistType int

const (
	PlaylistTypeMaster PlaylistType = iota
	PlaylistTypeMedia
)

// Track represents an audio or video track in a master playlist
type Track struct {
	Type       string // "AUDIO" or "VIDEO"
	URI        string
	GroupID    string
	Name       string
	Language   string
	IsDefault  bool
	Resolution string
	Bandwidth  int
	Codecs     string
}

// ParsePlaylistType determines if a playlist is master or media
func ParsePlaylistType(content string) PlaylistType {
	if strings.Contains(content, "#EXT-X-STREAM-INF") || strings.Contains(content, "#EXT-X-MEDIA:") {
		return PlaylistTypeMaster
	}
	return PlaylistTypeMedia
}

// ExtractTracksFromMaster extracts all audio and video tracks from a master playlist
func ExtractTracksFromMaster(content string) (audioTracks []Track, videoTracks []Track) {
	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Parse #EXT-X-MEDIA (audio tracks)
		if strings.HasPrefix(line, "#EXT-X-MEDIA:") {
			track := Track{}
			track.Type = extractAttribute(line, "TYPE")
			track.URI = extractAttribute(line, "URI")
			track.GroupID = extractAttribute(line, "GROUP-ID")
			track.Name = extractAttribute(line, "NAME")
			track.Language = extractAttribute(line, "LANGUAGE")
			track.IsDefault = extractAttribute(line, "DEFAULT") == "YES"

			if track.Type == "AUDIO" {
				audioTracks = append(audioTracks, track)
			}
		}

		// Parse #EXT-X-STREAM-INF (video tracks)
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			track := Track{}
			track.Type = "VIDEO"
			track.Resolution = extractAttribute(line, "RESOLUTION")
			track.Bandwidth = extractIntAttribute(line, "BANDWIDTH")
			track.Codecs = extractAttribute(line, "CODECS")

			// Next line is the URI
			if i+1 < len(lines) {
				track.URI = strings.TrimSpace(lines[i+1])
			}

			videoTracks = append(videoTracks, track)
		}
	}

	return audioTracks, videoTracks
}

// ResolveURL resolves a relative URL against a base URL
func ResolveURL(baseURL, relativeURL string) string {
	// If already absolute, return as-is
	if strings.HasPrefix(relativeURL, "http://") || strings.HasPrefix(relativeURL, "https://") {
		return relativeURL
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return relativeURL
	}

	rel, err := url.Parse(relativeURL)
	if err != nil {
		return relativeURL
	}

	return base.ResolveReference(rel).String()
}

// extractAttribute extracts an attribute value from an HLS tag line
func extractAttribute(line, attr string) string {
	// Look for ATTR="value" or ATTR=value
	pattern := attr + "="
	idx := strings.Index(line, pattern)
	if idx == -1 {
		return ""
	}

	start := idx + len(pattern)
	if start >= len(line) {
		return ""
	}

	// Check if value is quoted
	if line[start] == '"' {
		start++
		end := strings.Index(line[start:], "\"")
		if end == -1 {
			return ""
		}
		return line[start : start+end]
	}

	// Unquoted value - read until comma or end
	end := strings.IndexAny(line[start:], ",\n")
	if end == -1 {
		return line[start:]
	}
	return line[start : start+end]
}

// extractIntAttribute extracts an integer attribute value
func extractIntAttribute(line, attr string) int {
	val := extractAttribute(line, attr)
	var result int
	fmt.Sscanf(val, "%d", &result)
	return result
}
