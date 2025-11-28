package hls

import (
	"fmt"
	"net/url"
	"strings"
	"wails-cast/pkg/mediainfo"
)

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

// ExtractTracksFromMain extracts all audio and video tracks from a main playlist
// This is a convenience wrapper around the structured playlist parser
func ExtractTracksFromMain(playlist *MainPlaylist) (*mediainfo.MediaTrackInfo, error) {
	mi := mediainfo.MediaTrackInfo{
		VideoTracks:    make([]mediainfo.VideoTrack, 0),
		AudioTracks:    make([]mediainfo.AudioTrack, 0),
		SubtitleTracks: make([]mediainfo.SubtitleTrack, 0),
	}

	// Try to use the structured parser
	for i, variant := range playlist.VideoVariants {
		track := mediainfo.VideoTrack{
			URI:        variant.URI,
			Resolution: variant.Resolution,
			Bandwidth:  variant.Bandwidth,
			Codecs:     variant.Codecs,
			Index:      i,
		}

		// Extract first codec
		if variant.Codecs != "" {
			parts := strings.Split(variant.Codecs, ",")
			if len(parts) > 0 {
				track.Codec = strings.TrimSpace(parts[0])
			}
		}

		mi.VideoTracks = append(mi.VideoTracks, track)
	}

	// Flatten audio groups
	audioIdx := 0
	for _, audioTracks := range playlist.AudioGroups {
		for _, audio := range audioTracks {
			track := mediainfo.AudioTrack{
				URI:       audio.URI,
				GroupID:   audio.GroupID,
				Name:      audio.Name,
				Language:  audio.Language,
				IsDefault: audio.Default,
				Index:     audioIdx,
			}
			mi.AudioTracks = append(mi.AudioTracks, track)
			audioIdx++
		}
	}

	// Flatten subtitle groups
	subtitleIdx := 0
	for _, subtitleTracks := range playlist.SubtitleGroups {
		for _, subtitle := range subtitleTracks {
			track := mediainfo.SubtitleTrack{
				Title:    subtitle.Name,
				Language: subtitle.Language,
				Index:    subtitleIdx,
			}
			mi.SubtitleTracks = append(mi.SubtitleTracks, track)
			subtitleIdx++
		}

	}
	return &mi, nil
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
