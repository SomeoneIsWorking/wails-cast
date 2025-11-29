package hls

import (
	"net/url"
	"strings"
	"wails-cast/pkg/mediainfo"
)

// ExtractTracksFromManifest extracts all audio and video tracks from a manifest playlist
// This is a convenience wrapper around the structured playlist parser
func ExtractTracksFromManifest(playlist *ManifestPlaylist) (*mediainfo.MediaTrackInfo, error) {
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
