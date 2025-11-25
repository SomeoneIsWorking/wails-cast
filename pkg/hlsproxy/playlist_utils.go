package hlsproxy

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// rewriteManifest rewrites URLs in the manifest to point to the proxy
func (p *HLSProxy) rewriteManifest(manifest string, baseURL string, segmentPrefix string) string {
	isMaster := strings.Contains(manifest, "#EXT-X-STREAM-INF")
	lines := strings.Split(manifest, "\n")

	base, err := url.Parse(baseURL)
	if err != nil {
		fmt.Printf("Error parsing base URL %s: %v\n", baseURL, err)
		return manifest
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip if already rewritten
		if strings.Contains(trimmed, p.LocalIP) || strings.Contains(trimmed, "/segment/") || strings.Contains(trimmed, "/playlist/") || strings.Contains(trimmed, "/audio.m3u8") || strings.Contains(trimmed, "/video.m3u8") {
			continue
		}

		// Check for URI="..." pattern (e.g. for keys or subtitles)
		if strings.Contains(line, `URI="`) {
			uriPattern := regexp.MustCompile(`URI="([^"]+)"`)
			line = uriPattern.ReplaceAllStringFunc(line, func(match string) string {
				path := uriPattern.FindStringSubmatch(match)[1]

				// Resolve to absolute URL
				u, err := url.Parse(path)
				if err == nil {
					path = base.ResolveReference(u).String()
				}

				id := p.getOrAssignID(path, "")

				if isMaster {
					return fmt.Sprintf(`URI="/playlist/%s.m3u8"`, id)
				}

				// Determine extension based on path or default to .ts
				ext := filepath.Ext(path)
				if ext == "" {
					ext = ".ts"
				}
				return fmt.Sprintf(`URI="/segment/%s%s%s"`, segmentPrefix, id, ext)
			})
			lines[i] = line
			continue
		}

		// Check for segment URLs (lines not starting with #)
		if !strings.HasPrefix(trimmed, "#") {
			// Resolve to absolute URL
			u, err := url.Parse(trimmed)
			if err == nil {
				trimmed = base.ResolveReference(u).String()
			}

			id := p.getOrAssignID(trimmed, segmentPrefix)
			lines[i] = fmt.Sprintf("/segment/%s%s.ts", segmentPrefix, id)
		}
	}

	return strings.Join(lines, "\n")
}

// rewriteDemuxedMaster rewrites a demuxed master playlist to use /audio.m3u8 and /video.m3u8
func (p *HLSProxy) rewriteDemuxedMaster(manifest string, baseURL string) string {
	lines := strings.Split(manifest, "\n")
	var result []string
	expectingVideoURI := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}

		// Skip if already rewritten
		if strings.Contains(trimmed, p.LocalIP) || strings.Contains(trimmed, "/segment/") || strings.Contains(trimmed, "/playlist/") || strings.Contains(trimmed, "/audio.m3u8") || strings.Contains(trimmed, "/video.m3u8") {
			result = append(result, line)
			expectingVideoURI = false
			continue
		}

		// Check for URI="..." in EXT-X-MEDIA TYPE=AUDIO
		if strings.Contains(line, `TYPE=AUDIO`) && strings.Contains(line, `URI="`) {
			// Replace URI with /audio.m3u8
			uriPattern := regexp.MustCompile(`URI="([^"]+)"`)
			line = uriPattern.ReplaceAllString(line, `URI="/audio.m3u8"`)
			result = append(result, line)
			expectingVideoURI = false
			continue
		}

		// If expecting video URI and this is not a comment/tag, replace with /video.m3u8
		if expectingVideoURI && !strings.HasPrefix(trimmed, "#") {
			result = append(result, "/video.m3u8")
			expectingVideoURI = false
			continue
		}

		// Check for #EXT-X-STREAM-INF
		if strings.HasPrefix(trimmed, "#EXT-X-STREAM-INF") {
			expectingVideoURI = true
		} else {
			expectingVideoURI = false
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// ensureHLSTags adds required HLS tags if missing for better Chromecast compatibility
func (p *HLSProxy) ensureHLSTags(playlist string) string {
	lines := strings.Split(playlist, "\n")
	var result []string

	hasVersion := false
	hasTargetDuration := false
	hasMediaSequence := false
	isMediaPlaylist := false

	// First pass: check what we have
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-VERSION") {
			hasVersion = true
		} else if strings.HasPrefix(trimmed, "#EXT-X-TARGETDURATION") {
			hasTargetDuration = true
		} else if strings.HasPrefix(trimmed, "#EXT-X-MEDIA-SEQUENCE") {
			hasMediaSequence = true
		} else if strings.HasPrefix(trimmed, "#EXTINF") {
			isMediaPlaylist = true
		}
	}

	// If it's not a media playlist (it's a master playlist), don't add these tags
	if !isMediaPlaylist {
		return playlist
	}

	// Second pass: add missing tags
	inHeader := true
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Add to result
		result = append(result, line)

		// After #EXTM3U, add missing header tags
		if inHeader && strings.HasPrefix(trimmed, "#EXTM3U") {
			if !hasVersion {
				result = append(result, "#EXT-X-VERSION:3")
			}
			if !hasTargetDuration && isMediaPlaylist {
				result = append(result, "#EXT-X-TARGETDURATION:10")
			}
			if !hasMediaSequence && isMediaPlaylist {
				result = append(result, "#EXT-X-MEDIA-SEQUENCE:0")
			}
			inHeader = false
		}

		// Stop adding header tags after first segment
		if i > 0 && !strings.HasPrefix(trimmed, "#") && trimmed != "" {
			inHeader = false
		}
	}

	return strings.Join(result, "\n")
}
