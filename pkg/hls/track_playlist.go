package hls

import (
	"fmt"
	"strconv"
	"strings"
)

// TrackPlaylist represents an HLS media playlist
type TrackPlaylist struct {
	Version             int
	TargetDuration      int
	MediaSequence       int
	PlaylistType        string // VOD or EVENT
	Segments            []Segment
	Map                 *Map
	EndList             bool `default:"true"`
	IndependentSegments bool
}

// Segment represents a media segment
type Segment struct {
	Duration        float64
	Title           string
	URI             string
	ByteRange       *ByteRange
	Discontinuity   bool
	Key             *Key // Encryption key (if different from playlist-level)
	ProgramDateTime string
	Attrs           map[string]string
}

// parseTrackPlaylist parses a media playlist
func parseTrackPlaylist(lines []string) (*TrackPlaylist, error) {
	media := &TrackPlaylist{
		Segments: make([]Segment, 0),
	}

	var currentKey *Key
	var nextSegment *Segment

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "#EXT-X-VERSION:") {
			media.Version, _ = strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-VERSION:"))
		} else if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			media.TargetDuration, _ = strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:"))
		} else if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			media.MediaSequence, _ = strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"))
		} else if strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE:") {
			media.PlaylistType = strings.TrimPrefix(line, "#EXT-X-PLAYLIST-TYPE:")
		} else if strings.HasPrefix(line, "#EXT-X-INDEPENDENT-SEGMENTS") {
			media.IndependentSegments = true
		} else if strings.HasPrefix(line, "#EXT-X-ENDLIST") {
			media.EndList = true
		} else if strings.HasPrefix(line, "#EXT-X-KEY:") {
			key := parseKey(line)
			currentKey = &key
		} else if strings.HasPrefix(line, "#EXT-X-MAP:") {
			media.Map = parseMap(line)
		} else if strings.HasPrefix(line, "#EXTINF:") {
			// Parse duration and title
			content := strings.TrimPrefix(line, "#EXTINF:")
			parts := strings.SplitN(content, ",", 2)

			duration, _ := strconv.ParseFloat(parts[0], 64)
			title := ""
			if len(parts) > 1 {
				title = parts[1]
			}

			nextSegment = &Segment{
				Duration: duration,
				Title:    title,
				Key:      currentKey,
				Attrs:    make(map[string]string),
			}
		} else if strings.HasPrefix(line, "#EXT-X-DISCONTINUITY") {
			if nextSegment != nil {
				nextSegment.Discontinuity = true
			}
		} else if strings.HasPrefix(line, "#EXT-X-BYTERANGE:") {
			if nextSegment != nil {
				nextSegment.ByteRange = parseByteRange(strings.TrimPrefix(line, "#EXT-X-BYTERANGE:"))
			}
		} else if strings.HasPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:") {
			if nextSegment != nil {
				nextSegment.ProgramDateTime = strings.TrimPrefix(line, "#EXT-X-PROGRAM-DATE-TIME:")
			}
		} else if !strings.HasPrefix(line, "#") {
			// This is a URI line
			if nextSegment != nil {
				nextSegment.URI = line
				media.Segments = append(media.Segments, *nextSegment)
				nextSegment = nil
			}
		}
	}

	return media, nil
}

// Generate creates an HLS media playlist string
func (m *TrackPlaylist) Generate() string {
	var lines []string

	lines = append(lines, "#EXTM3U")

	if m.Version > 0 {
		lines = append(lines, fmt.Sprintf("#EXT-X-VERSION:%d", m.Version))
	}

	if m.TargetDuration > 0 {
		lines = append(lines, fmt.Sprintf("#EXT-X-TARGETDURATION:%d", m.TargetDuration))
	}

	if m.MediaSequence > 0 {
		lines = append(lines, fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d", m.MediaSequence))
	}

	if m.PlaylistType != "" {
		lines = append(lines, fmt.Sprintf("#EXT-X-PLAYLIST-TYPE:%s", m.PlaylistType))
	}

	if m.IndependentSegments {
		lines = append(lines, "#EXT-X-INDEPENDENT-SEGMENTS")
	}

	// Add map if present
	if m.Map != nil {
		mapLine := fmt.Sprintf(`#EXT-X-MAP:URI="%s"`, m.Map.URI)
		if m.Map.ByteRange != nil {
			mapLine += fmt.Sprintf(`,BYTERANGE="%d@%d"`, m.Map.ByteRange.Length, m.Map.ByteRange.Offset)
		}
		lines = append(lines, mapLine)
	}

	// Add segments
	var lastKey *Key
	for _, segment := range m.Segments {
		// Add key if changed
		if segment.Key != nil && (lastKey == nil || *segment.Key != *lastKey) {
			keyLine := fmt.Sprintf(`#EXT-X-KEY:METHOD=%s`, segment.Key.Method)
			if segment.Key.URI != "" {
				keyLine += fmt.Sprintf(`,URI="%s"`, segment.Key.URI)
			}
			if segment.Key.IV != "" {
				keyLine += fmt.Sprintf(`,IV=%s`, segment.Key.IV)
			}
			lines = append(lines, keyLine)
			lastKey = segment.Key
		}

		if segment.Discontinuity {
			lines = append(lines, "#EXT-X-DISCONTINUITY")
		}

		if segment.ProgramDateTime != "" {
			lines = append(lines, fmt.Sprintf("#EXT-X-PROGRAM-DATE-TIME:%s", segment.ProgramDateTime))
		}

		// EXTINF
		lines = append(lines, fmt.Sprintf("#EXTINF:%.6f,%s", segment.Duration, segment.Title))

		if segment.ByteRange != nil {
			lines = append(lines, fmt.Sprintf("#EXT-X-BYTERANGE:%d@%d", segment.ByteRange.Length, segment.ByteRange.Offset))
		}

		lines = append(lines, segment.URI)
	}

	if m.EndList {
		lines = append(lines, "#EXT-X-ENDLIST")
	}

	return strings.Join(lines, "\n") + "\n"
}
