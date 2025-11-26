package hls

import (
	"fmt"
	"net/url"
	"strings"
	"wails-cast/pkg/mediainfo"
)

// PlaylistType represents the type of HLS playlist
type PlaylistType int

const (
	PlaylistTypeMaster PlaylistType = iota
	PlaylistTypeMedia
)

// ParsePlaylistType determines if a playlist is master or media
func ParsePlaylistType(content string) PlaylistType {
	if strings.Contains(content, "#EXT-X-STREAM-INF") || strings.Contains(content, "#EXT-X-MEDIA:") {
		return PlaylistTypeMaster
	}
	return PlaylistTypeMedia
}

// GenerateVODPlaylist generates a complete HLS VOD playlist
func GenerateVODPlaylist(duration float64, segmentSize int, localIP string, port int) string {
	var playlist strings.Builder

	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:3\n")
	playlist.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", segmentSize))
	playlist.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	playlist.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	// Calculate number of segments
	numSegments := int(duration / float64(segmentSize))
	if float64(numSegments*segmentSize) < duration {
		numSegments++
	}

	// Add all segments with proper durations
	for i := 0; i < numSegments; i++ {
		segmentDuration := float64(segmentSize)
		// Last segment might be shorter
		if i == numSegments-1 {
			remaining := duration - float64(i*segmentSize)
			if remaining < float64(segmentSize) {
				segmentDuration = remaining
			}
		}

		playlist.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segmentDuration))
		playlist.WriteString(fmt.Sprintf("http://%s:%d/segment%d.ts\n", localIP, port, i))
	}

	playlist.WriteString("#EXT-X-ENDLIST\n")
	return playlist.String()
}

// ExtractTracksFromMaster extracts all audio and video tracks from a master playlist
func ExtractTracksFromMaster(content string) mediainfo.MediaTrackInfo {
	lines := strings.Split(content, "\n")

	mi := mediainfo.MediaTrackInfo{
		VideoTracks:    make([]mediainfo.VideoTrack, 0),
		AudioTracks:    make([]mediainfo.AudioTrack, 0),
		SubtitleTracks: make([]mediainfo.SubtitleTrack, 0),
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Parse #EXT-X-MEDIA (audio tracks)
		if strings.HasPrefix(line, "#EXT-X-MEDIA:") {
			_type := extractAttribute(line, "TYPE")

			switch _type {
			case "AUDIO":
				track := mediainfo.AudioTrack{}
				track.URI = extractAttribute(line, "URI")
				track.GroupID = extractAttribute(line, "GROUP-ID")
				track.Name = extractAttribute(line, "NAME")
				track.Language = extractAttribute(line, "LANGUAGE")
				track.IsDefault = extractAttribute(line, "DEFAULT") == "YES"
				track.Index = len(mi.AudioTracks)
				mi.AudioTracks = append(mi.AudioTracks, track)

			case "SUBTITLES":
				track := mediainfo.SubtitleTrack{}
				track.Title = extractAttribute(line, "NAME")
				track.Language = extractAttribute(line, "LANGUAGE")
				track.Index = len(mi.SubtitleTracks)
				mi.SubtitleTracks = append(mi.SubtitleTracks, track)
			}
		}

		// Parse #EXT-X-STREAM-INF (video tracks)
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			track := mediainfo.VideoTrack{}
			track.Type = "VIDEO"
			track.Resolution = extractAttribute(line, "RESOLUTION")
			track.Bandwidth = extractIntAttribute(line, "BANDWIDTH")
			track.Codecs = extractAttribute(line, "CODECS")

			// Next line is the URI
			if i+1 < len(lines) {
				track.URI = strings.TrimSpace(lines[i+1])
			}

			codec := ""
			if track.Codecs != "" {
				// Take first codec before comma
				parts := strings.Split(track.Codecs, ",")
				if len(parts) > 0 {
					codec = strings.TrimSpace(parts[0])
				}
			}
			track.Codec = codec
			track.Index = len(mi.VideoTracks)
			mi.VideoTracks = append(mi.VideoTracks, track)
		}
	}

	return mi
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

// CalculateTotalDuration calculates the total duration of a video M3U8 playlist by summing all segment durations
func CalculateTotalDuration(content string) float64 {
	lines := strings.Split(content, "\n")
	var total float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			// Extract duration from #EXTINF:duration,
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				durationStr := strings.TrimSuffix(parts[1], ",")
				var dur float64
				fmt.Sscanf(durationStr, "%f", &dur)
				total += dur
			}
		}
	}

	return total
}
