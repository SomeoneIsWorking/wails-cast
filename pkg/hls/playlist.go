package hls

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Playlist represents a parsed HLS playlist (master or media)
type Playlist struct {
	Type     PlaylistType
	Main     *MainPlaylist
	Track    *TrackPlaylist
	RawLines []string // Original lines for unparsed tags
}

// PlaylistType represents the type of HLS playlist
type PlaylistType int

const (
	PlaylistTypeMain PlaylistType = iota
	PlaylistTypeTrack
)

// MainPlaylist represents an HLS master playlist
type MainPlaylist struct {
	Version             int
	VideoVariants       []VideoVariant
	AudioGroups         map[string][]AudioMedia    // Group ID -> Audio tracks
	SubtitleGroups      map[string][]SubtitleMedia // Group ID -> Subtitle tracks
	IndependentSegments bool
}

func (m *MainPlaylist) Clone() *MainPlaylist {
	_json, _ := json.Marshal(m)
	clone := &MainPlaylist{}
	json.Unmarshal(_json, clone)
	return clone
}

// VideoVariant represents a video stream variant (#EXT-X-STREAM-INF)
type VideoVariant struct {
	URI        string
	Bandwidth  int
	Codecs     string
	Resolution string
	FrameRate  float64
	Audio      string            // Audio group ID
	Subtitles  string            // Subtitle group ID
	Attrs      map[string]string // Other attributes
	Index      int               // Index in the master playlist
}

// AudioMedia represents an audio track (#EXT-X-MEDIA TYPE=AUDIO)
type AudioMedia struct {
	URI        string
	GroupID    string
	Name       string
	Language   string
	Default    bool
	Autoselect bool
	Channels   string
	Attrs      map[string]string
	Index      int
}

// SubtitleMedia represents a subtitle track (#EXT-X-MEDIA TYPE=SUBTITLES)
type SubtitleMedia struct {
	URI        string
	GroupID    string
	Name       string
	Language   string
	Default    bool
	Autoselect bool
	Forced     bool
	Attrs      map[string]string
	Index      int
}

// TrackPlaylist represents an HLS media playlist
type TrackPlaylist struct {
	Version             int
	TargetDuration      int
	MediaSequence       int
	PlaylistType        string // VOD or EVENT
	Segments            []Segment
	Map                 *Map
	EndList             bool
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

// ParsePlaylistType determines if a playlist is master or media
func parsePlaylistType(content string) PlaylistType {
	if strings.Contains(content, "#EXT-X-STREAM-INF") || strings.Contains(content, "#EXT-X-MEDIA:") {
		return PlaylistTypeMain
	}
	return PlaylistTypeTrack
}

// ParsePlaylist parses an HLS playlist string into a structured Playlist
func parsePlaylist(content string) (*Playlist, error) {
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

	playlist := &Playlist{
		RawLines: cleanLines,
	}

	// Determine playlist type
	playlist.Type = parsePlaylistType(content)

	if playlist.Type == PlaylistTypeMain {
		master, err := parseMasterPlaylist(cleanLines)
		if err != nil {
			return nil, err
		}
		playlist.Main = master
	} else {
		track, err := parseTrackPlaylist(cleanLines)
		if err != nil {
			return nil, err
		}
		playlist.Track = track
	}

	return playlist, nil
}

// ParseMainPlaylist parses a main (master) playlist
func ParseMainPlaylist(content string) (*MainPlaylist, error) {
	playlist, err := parsePlaylist(content)
	if err != nil {
		return nil, err
	}
	if playlist.Type != PlaylistTypeMain {
		return nil, fmt.Errorf("not a main playlist")
	}
	return playlist.Main, nil
}

// ParseTrackPlaylist parses a track (media) playlist
func ParseTrackPlaylist(content string) (*TrackPlaylist, error) {
	playlist, err := parsePlaylist(content)
	if err != nil {
		return nil, err
	}
	if playlist.Type != PlaylistTypeTrack {
		return nil, fmt.Errorf("not a track playlist")
	}
	return playlist.Track, nil
}

// parseMasterPlaylist parses a master playlist
func parseMasterPlaylist(lines []string) (*MainPlaylist, error) {
	master := &MainPlaylist{
		AudioGroups:    make(map[string][]AudioMedia),
		SubtitleGroups: make(map[string][]SubtitleMedia),
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "#EXT-X-VERSION:") {
			master.Version, _ = strconv.Atoi(strings.TrimPrefix(line, "#EXT-X-VERSION:"))
		} else if strings.HasPrefix(line, "#EXT-X-INDEPENDENT-SEGMENTS") {
			master.IndependentSegments = true
		} else if strings.HasPrefix(line, "#EXT-X-MEDIA:") {
			mediaType := extractAttribute(line, "TYPE")

			switch mediaType {
			case "AUDIO":
				groupId := extractAttribute(line, "GROUP-ID")
				audio := AudioMedia{
					URI:        extractAttribute(line, "URI"),
					GroupID:    groupId,
					Name:       extractAttribute(line, "NAME"),
					Language:   extractAttribute(line, "LANGUAGE"),
					Default:    extractAttribute(line, "DEFAULT") == "YES",
					Autoselect: extractAttribute(line, "AUTOSELECT") == "YES",
					Channels:   extractAttribute(line, "CHANNELS"),
					Attrs:      parseAttributes(line),
					Index:      len(master.AudioGroups[groupId]),
				}
				master.AudioGroups[audio.GroupID] = append(master.AudioGroups[audio.GroupID], audio)
			case "SUBTITLES":
				groupId := extractAttribute(line, "GROUP-ID")
				subtitle := SubtitleMedia{
					URI:        extractAttribute(line, "URI"),
					GroupID:    groupId,
					Name:       extractAttribute(line, "NAME"),
					Language:   extractAttribute(line, "LANGUAGE"),
					Default:    extractAttribute(line, "DEFAULT") == "YES",
					Autoselect: extractAttribute(line, "AUTOSELECT") == "YES",
					Forced:     extractAttribute(line, "FORCED") == "YES",
					Attrs:      parseAttributes(line),
					Index:      len(master.SubtitleGroups[groupId]),
				}
				master.SubtitleGroups[subtitle.GroupID] = append(master.SubtitleGroups[subtitle.GroupID], subtitle)
			}
		} else if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			variant := VideoVariant{
				Bandwidth:  extractIntAttribute(line, "BANDWIDTH"),
				Codecs:     extractAttribute(line, "CODECS"),
				Resolution: extractAttribute(line, "RESOLUTION"),
				Audio:      extractAttribute(line, "AUDIO"),
				Subtitles:  extractAttribute(line, "SUBTITLES"),
				Attrs:      parseAttributes(line),
				Index:      len(master.VideoVariants),
			}

			// Frame rate
			if fr := extractAttribute(line, "FRAME-RATE"); fr != "" {
				variant.FrameRate, _ = strconv.ParseFloat(fr, 64)
			}

			// Next line is the URI
			if i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "#") {
				i++
				variant.URI = lines[i]
			}

			master.VideoVariants = append(master.VideoVariants, variant)
		}
	}

	return master, nil
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

// Generate converts a Playlist struct back to HLS format string
func (p *Playlist) Generate() string {
	if p.Type == PlaylistTypeMain && p.Main != nil {
		return p.Main.Generate()
	} else if p.Type == PlaylistTypeTrack && p.Track != nil {
		return p.Track.Generate()
	}
	return ""
}

// Generate creates an HLS master playlist string
func (m *MainPlaylist) Generate() string {
	var lines []string

	lines = append(lines, "#EXTM3U")

	if m.Version > 0 {
		lines = append(lines, fmt.Sprintf("#EXT-X-VERSION:%d", m.Version))
	}

	if m.IndependentSegments {
		lines = append(lines, "#EXT-X-INDEPENDENT-SEGMENTS")
	}

	// Write audio media
	for _, audioTracks := range m.AudioGroups {
		for _, audio := range audioTracks {
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
			if audio.URI != "" {
				attrs = append(attrs, fmt.Sprintf(`URI="%s"`, audio.URI))
			}
			if audio.Channels != "" {
				attrs = append(attrs, fmt.Sprintf(`CHANNELS="%s"`, audio.Channels))
			}

			lines = append(lines, "#EXT-X-MEDIA:"+strings.Join(attrs, ","))
		}
	}

	// Write subtitle media
	for _, subtitleTracks := range m.SubtitleGroups {
		for _, subtitle := range subtitleTracks {
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
			if subtitle.URI != "" {
				attrs = append(attrs, fmt.Sprintf(`URI="%s"`, subtitle.URI))
			}

			lines = append(lines, "#EXT-X-MEDIA:"+strings.Join(attrs, ","))
		}
	}

	// Write video variants
	for _, variant := range m.VideoVariants {
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
		lines = append(lines, variant.URI)
	}

	return strings.Join(lines, "\n") + "\n"
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
