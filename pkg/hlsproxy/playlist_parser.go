package hlsproxy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// downloadAndParseNestedPlaylists downloads audio and video playlists and caches them
func (p *HLSProxy) downloadAndParseNestedPlaylists() {
	fmt.Println("Downloading nested playlists to cache them...")

	// Download audio playlist
	if p.AudioPlaylistURL != "" {
		resp, err := p.downloadFile(p.AudioPlaylistURL)
		if err != nil {
			fmt.Printf("Failed to download audio playlist: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		audioPlaylist := string(body)

		// Save audio playlist to cache
		audioLocalPath := filepath.Join(p.CacheDir, "audio.m3u8")
		if err := os.WriteFile(audioLocalPath, []byte(audioPlaylist), 0644); err == nil {
			audioItem := &ManifestItem{
				URL:         p.AudioPlaylistURL,
				ContentType: "application/vnd.apple.mpegurl",
				LocalPath:   audioLocalPath,
				IsPlaylist:  true,
			}
			p.updateManifest(p.AudioPlaylistURL, audioItem)
		}

		fmt.Printf("Cached audio playlist as audio.m3u8\n")
	}

	// Download video playlist
	if p.VideoPlaylistURL != "" {
		resp, err := p.downloadFile(p.VideoPlaylistURL)
		if err != nil {
			fmt.Printf("Failed to download video playlist: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		videoPlaylist := string(body)

		// Save video playlist to cache
		videoLocalPath := filepath.Join(p.CacheDir, "video.m3u8")
		if err := os.WriteFile(videoLocalPath, []byte(videoPlaylist), 0644); err == nil {
			videoItem := &ManifestItem{
				URL:         p.VideoPlaylistURL,
				ContentType: "application/vnd.apple.mpegurl",
				LocalPath:   videoLocalPath,
				IsPlaylist:  true,
			}
			p.updateManifest(p.VideoPlaylistURL, videoItem)
		}

		fmt.Printf("Cached video playlist as video.m3u8\n")
	}
}
