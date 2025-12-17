package hls

import (
	"fmt"
	"net/url"
	"strings"
)

// playlist represents a parsed HLS playlist (manifest or track)
type playlist struct {
	Type     playlistType
	Manifest *ManifestPlaylist
	Track    *TrackPlaylist
	RawLines []string // Original lines for unparsed tags
}

// playlistType represents the type of HLS playlist
type playlistType int

const (
	PlaylistTypeManifest playlistType = iota
	PlaylistTypeTrack
)

// ManifestPlaylist represents an HLS manifest playlist
type ManifestPlaylist struct {
	Version             int
	VideoTracks         []VideoTrack
	AudioTracks         []AudioTrack    // Flat list of all audio tracks
	SubtitleTracks      []SubtitleTrack // Flat list of all subtitle trackss
	IndependentSegments bool
}

// VideoTrack represents a video stream variant (#EXT-X-STREAM-INF)
type VideoTrack struct {
	URI        *url.URL
	Bandwidth  int
	Codecs     string
	Resolution string
	FrameRate  float64
	Audio      string            // Audio group ID
	Subtitles  string            // Subtitle group ID
	Attrs      map[string]string // Other attributes
	Index      int               // Index in the manifest playlist
}

// AudioTrack represents an audio track (#EXT-X-MEDIA TYPE=AUDIO)
type AudioTrack struct {
	URI        *url.URL
	GroupID    string
	Name       string
	Language   string
	Default    bool
	Autoselect bool
	Channels   string
	Attrs      map[string]string
	Index      int
}

// SubtitleTrack represents a subtitle track (#EXT-X-MEDIA TYPE=SUBTITLES)
type SubtitleTrack struct {
	URI        *url.URL
	GroupID    string
	Name       string
	Language   string
	Default    bool
	Autoselect bool
	Forced     bool
	Attrs      map[string]string
	Index      int
}

// Key represents encryption information (#EXT-X-KEY)
type Key struct {
	Method            string
	URI               string
	IV                string
	KeyFormat         string
	KeyFormatVersions string
}

// Map represents init segment information (#EXT-X-MAP)
type Map struct {
	URI       string
	ByteRange *ByteRange
}

// ByteRange represents a byte range
type ByteRange struct {
	Length int64
	Offset int64
}

// ParsePlaylistType determines if a playlist is manifest or track
func parsePlaylistType(content string) playlistType {
	if strings.Contains(content, "#EXT-X-STREAM-INF") || strings.Contains(content, "#EXT-X-MEDIA:") {
		return PlaylistTypeManifest
	}
	return PlaylistTypeTrack
}

// ParsePlaylist parses an HLS playlist string into a structured Playlist
func parsePlaylist(content string, expectedType playlistType) (*playlist, error) {
	lines := strings.Split(content, "\n")

	// Clean lines
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	if len(cleanLines) == 0 || cleanLines[0] != "#EXTM3U" {
		return nil, fmt.Errorf("invalid playlist: missing #EXTM3U header")
	}

	playlist := &playlist{
		RawLines: cleanLines,
	}

	// Determine playlist type
	playlistType := parsePlaylistType(content)

	if playlistType != expectedType {
		return nil, fmt.Errorf("playlist type mismatch")
	}

	playlist.Type = playlistType

	if playlist.Type == PlaylistTypeManifest {
		manifest, err := parseManifestPlaylist(cleanLines)
		if err != nil {
			return nil, err
		}
		playlist.Manifest = manifest
	} else {
		track, err := parseTrackPlaylist(cleanLines)
		if err != nil {
			return nil, err
		}
		playlist.Track = track
	}

	return playlist, nil
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
		end := strings.Index(line[start:], `"`)
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
