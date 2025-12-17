package hls

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"wails-cast/pkg/urlhelper"
)

// ParseManifestPlaylist parses a manifest playlist
func ParseManifestPlaylist(content string) (*ManifestPlaylist, error) {
	playlist, err := parsePlaylist(content, PlaylistTypeManifest)
	if err != nil {
		return nil, err
	}
	return playlist.Manifest, nil
}

// ParseTrackPlaylist parses a track (media) playlist
func ParseTrackPlaylist(content string) (*TrackPlaylist, error) {
	playlist, err := parsePlaylist(content, PlaylistTypeTrack)
	if err != nil {
		return nil, err
	}
	return playlist.Track, nil
}

// parseManifestPlaylist parses a manifest playlist
func parseManifestPlaylist(lines []string) (*ManifestPlaylist, error) {
	manifest := &ManifestPlaylist{
		VideoTracks:    []VideoTrack{},
		AudioTracks:    []AudioTrack{},
		SubtitleTracks: []SubtitleTrack{},
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "#EXT-X-VERSION:") {
			manifest.Version, _ = strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-VERSION:"))
		} else if strings.HasPrefix(line, "#EXT-X-INDEPENDENT-SEGMENTS") {
			manifest.IndependentSegments = true
		} else if strings.HasPrefix(line, "#EXT-X-MEDIA:") {
			mediaType := extractAttribute(line, "TYPE")

			switch mediaType {
			case "AUDIO":
				groupId := extractAttribute(line, "GROUP-ID")
				audio := AudioTrack{
					URI:        urlhelper.Parse(extractAttribute(line, "URI")),
					GroupID:    groupId,
					Name:       extractAttribute(line, "NAME"),
					Language:   extractAttribute(line, "LANGUAGE"),
					Default:    extractAttribute(line, "DEFAULT") == "YES",
					Autoselect: extractAttribute(line, "AUTOSELECT") == "YES",
					Channels:   extractAttribute(line, "CHANNELS"),
					Attrs:      parseAttributes(line),
					Index:      len(manifest.AudioTracks),
				}
				manifest.AudioTracks = append(manifest.AudioTracks, audio)
			case "SUBTITLES":
				groupId := extractAttribute(line, "GROUP-ID")
				subtitle := SubtitleTrack{
					URI:        urlhelper.Parse(extractAttribute(line, "URI")),
					GroupID:    groupId,
					Name:       extractAttribute(line, "NAME"),
					Language:   extractAttribute(line, "LANGUAGE"),
					Default:    extractAttribute(line, "DEFAULT") == "YES",
					Autoselect: extractAttribute(line, "AUTOSELECT") == "YES",
					Forced:     extractAttribute(line, "FORCED") == "YES",
					Attrs:      parseAttributes(line),
					Index:      len(manifest.SubtitleTracks),
				}
				manifest.SubtitleTracks = append(manifest.SubtitleTracks, subtitle)
			}
		} else if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			variant := VideoTrack{
				Bandwidth:  extractIntAttribute(line, "BANDWIDTH"),
				Codecs:     extractAttribute(line, "CODECS"),
				Resolution: extractAttribute(line, "RESOLUTION"),
				Audio:      extractAttribute(line, "AUDIO"),
				Subtitles:  extractAttribute(line, "SUBTITLES"),
				Attrs:      parseAttributes(line),
				Index:      len(manifest.VideoTracks),
			}

			// Frame rate
			if fr := extractAttribute(line, "FRAME-RATE"); fr != "" {
				variant.FrameRate, _ = strconv.ParseFloat(fr, 64)
			}

			// Next line is the URI
			if i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "#") {
				i++
				variant.URI = urlhelper.Parse(lines[i])
			}

			manifest.VideoTracks = append(manifest.VideoTracks, variant)
		}
	}

	return manifest, nil
}

// Generate converts a Playlist struct back to HLS format string
func (p *playlist) Generate() string {
	if p.Type == PlaylistTypeManifest && p.Manifest != nil {
		return p.Manifest.Generate()
	} else if p.Type == PlaylistTypeTrack && p.Track != nil {
		return p.Track.Generate()
	}
	return ""
}

// Generate creates an HLS manifest playlist string
func (m *ManifestPlaylist) Generate() string {
	var lines []string

	lines = append(lines, "#EXTM3U")

	if m.Version > 0 {
		lines = append(lines, fmt.Sprintf("#EXT-X-VERSION:%d", m.Version))
	}

	if m.IndependentSegments {
		lines = append(lines, "#EXT-X-INDEPENDENT-SEGMENTS")
	}

	// Write audio media
	for _, audio := range m.AudioTracks {
		attrs := []string{
			`TYPE=AUDIO`,
			fmt.Sprintf(`GROUP-ID="%s"`, audio.GroupID),
			fmt.Sprintf(`NAME="%s"`, audio.Name),
		}

		if audio.Language != "" {
			attrs = append(attrs, fmt.Sprintf(`LANGUAGE="%s"`, audio.Language))
		}
		if audio.Default {
			attrs = append(attrs, `DEFAULT=YES`)
		}
		if audio.Autoselect {
			attrs = append(attrs, `AUTOSELECT=YES`)
		}
		if audio.URI != nil {
			attrs = append(attrs, fmt.Sprintf(`URI="%s"`, audio.URI))
		}
		if audio.Channels != "" {
			attrs = append(attrs, fmt.Sprintf(`CHANNELS="%s"`, audio.Channels))
		}

		lines = append(lines, "#EXT-X-MEDIA:"+strings.Join(attrs, ","))
	}

	// Write subtitle media
	for _, subtitle := range m.SubtitleTracks {
		attrs := []string{
			`TYPE=SUBTITLES`,
			fmt.Sprintf(`GROUP-ID="%s"`, subtitle.GroupID),
			fmt.Sprintf(`NAME="%s"`, subtitle.Name),
		}

		if subtitle.Language != "" {
			attrs = append(attrs, fmt.Sprintf(`LANGUAGE="%s"`, subtitle.Language))
		}
		if subtitle.Default {
			attrs = append(attrs, `DEFAULT=YES`)
		}
		if subtitle.Autoselect {
			attrs = append(attrs, `AUTOSELECT=YES`)
		}
		if subtitle.Forced {
			attrs = append(attrs, `FORCED=YES`)
		}
		if subtitle.URI != nil {
			attrs = append(attrs, fmt.Sprintf(`URI="%s"`, subtitle.URI))
		}

		lines = append(lines, "#EXT-X-MEDIA:"+strings.Join(attrs, ","))
	}

	// Write video variants
	for _, variant := range m.VideoTracks {
		attrs := []string{
			fmt.Sprintf(`BANDWIDTH=%d`, variant.Bandwidth),
		}

		if variant.Codecs != "" {
			attrs = append(attrs, fmt.Sprintf(`CODECS="%s"`, variant.Codecs))
		}
		if variant.Resolution != "" {
			attrs = append(attrs, fmt.Sprintf(`RESOLUTION=%s`, variant.Resolution))
		}
		if variant.FrameRate > 0 {
			attrs = append(attrs, fmt.Sprintf(`FRAME-RATE=%.3f`, variant.FrameRate))
		}
		if variant.Audio != "" {
			attrs = append(attrs, fmt.Sprintf(`AUDIO="%s"`, variant.Audio))
		}
		if variant.Subtitles != "" {
			attrs = append(attrs, fmt.Sprintf(`SUBTITLES="%s"`, variant.Subtitles))
		}

		lines = append(lines, "#EXT-X-STREAM-INF:"+strings.Join(attrs, ","))
		lines = append(lines, variant.URI.String())
	}

	return strings.Join(lines, "\n") + "\n"
}

// Helper parsing functions

func parseKey(line string) Key {
	return Key{
		Method:            extractAttribute(line, "METHOD"),
		URI:               extractAttribute(line, "URI"),
		IV:                extractAttribute(line, "IV"),
		KeyFormat:         extractAttribute(line, "KEYFORMAT"),
		KeyFormatVersions: extractAttribute(line, "KEYFORMATVERSIONS"),
	}
}

func parseMap(line string) *Map {
	return &Map{
		URI:       extractAttribute(line, "URI"),
		ByteRange: parseByteRangeAttr(extractAttribute(line, "BYTERANGE")),
	}
}

func parseByteRange(s string) *ByteRange {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, "@")
	length, _ := strconv.ParseInt(parts[0], 10, 64)
	offset := int64(0)
	if len(parts) > 1 {
		offset, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	return &ByteRange{Length: length, Offset: offset}
}

func parseByteRangeAttr(s string) *ByteRange {
	return parseByteRange(s)
}

func parseAttributes(line string) map[string]string {
	attrs := make(map[string]string)

	// Simple attribute parsing - can be enhanced
	re := regexp.MustCompile(`([A-Z-]+)=("[^"]*"|[^,]+)`)
	matches := re.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) == 3 {
			key := match[1]
			value := strings.Trim(match[2], `"`)
			attrs[key] = value
		}
	}

	return attrs
}
