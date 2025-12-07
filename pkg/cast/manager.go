package cast

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"wails-cast/pkg/extractor"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/logger"
	"wails-cast/pkg/mediainfo"
	"wails-cast/pkg/options"
	"wails-cast/pkg/stream"
)

// CastManager handles the casting workflow including caching and proxying
type CastManager struct {
	LocalIP   string
	ProxyPort int
	CacheRoot string
}

// NewCastManager creates a new CastManager
func NewCastManager(localIP string, proxyPort int) *CastManager {
	cacheRoot := folders.GetCache()
	os.MkdirAll(cacheRoot, 0755)
	return &CastManager{
		LocalIP:   localIP,
		ProxyPort: proxyPort,
		CacheRoot: cacheRoot,
	}
}

func (m *CastManager) CreateRemoteHandler(videoURL string, options *options.StreamOptions) (*stream.RemoteHandler, error) {
	// 1. Calculate hash of video URL for cache key
	cacheDir := m.cacheDir(videoURL)

	// 2. Check cache or extract
	result, err := getExtractionJson(videoURL, cacheDir)
	if err != nil {
		return nil, err
	}

	// Create and configure handler
	handler := stream.NewRemoteHandler(m.LocalIP, cacheDir, options)
	handler.SetExtractor(result)
	trackPlaylist, err := handler.GetTrackPlaylist(context.Background(), "video", 0)
	if err != nil {
		return nil, err
	}
	handler.Duration = 0.0
	for _, segment := range trackPlaylist.Segments {
		handler.Duration += segment.Duration
	}
	return handler, nil
}

func (m *CastManager) cacheDir(videoURL string) string {
	hash := md5.Sum([]byte(videoURL))
	cacheKey := hex.EncodeToString(hash[:])
	cacheDir := filepath.Join(m.CacheRoot, cacheKey)
	return cacheDir
}

func loadCachedExtraction(cacheDir string) (*extractor.ExtractResult, error) {
	extractionFile := filepath.Join(cacheDir, "extraction.json")

	if _, err := os.Stat(extractionFile); err != nil {
		return nil, err
	}

	logger.Logger.Info("Found cached extraction, loading...")
	data, err := os.ReadFile(extractionFile)
	if err != nil {
		return nil, err
	}

	result := &extractor.ExtractResult{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, fmt.Errorf("error unmarshaling cached extraction: %w", err)
	}

	// Load ManifestRaw from file
	playlistFile := filepath.Join(cacheDir, "playlist_raw.m3u8")
	if manifestData, err := os.ReadFile(playlistFile); err == nil {
		result.ManifestRaw = string(manifestData)
	}

	return result, nil
}

func getExtractionJson(videoURL string, cacheDir string) (*extractor.ExtractResult, error) {
	// Try loading from cache first
	if result, err := loadCachedExtraction(cacheDir); err == nil {
		return result, nil
	}

	// Cache miss - extract fresh
	fmt.Printf("Extracting video from: %s\n", videoURL)
	fmt.Println("Please click the PLAY button in the browser window...")

	result, err := extractor.ExtractManifestPlaylist(videoURL)
	if err != nil {
		return nil, fmt.Errorf("error extracting video: %w", err)
	}

	// Save extraction result
	os.MkdirAll(cacheDir, 0755)

	// Save raw playlist
	playlistFile := filepath.Join(cacheDir, "playlist_raw.m3u8")
	os.WriteFile(playlistFile, []byte(result.ManifestRaw), 0644)

	// Save subtitles as separate files
	for i, subtitle := range result.Subtitles {
		subtitleFile := filepath.Join(cacheDir, fmt.Sprintf("subtitle_%d.vtt", i))
		os.WriteFile(subtitleFile, []byte(subtitle.Content), 0644)
	}

	// Save extraction metadata (json:"-" tags exclude ManifestRaw and Content)
	extractionFile := filepath.Join(cacheDir, "extraction.json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err == nil {
		os.WriteFile(extractionFile, data, 0644)
	}
	return result, nil
}

// GetRemoteTrackInfo extracts track information from a remote HLS stream
func (m *CastManager) GetRemoteTrackInfo(videoURL string) (*mediainfo.MediaTrackInfo, error) {
	result, err := getExtractionJson(videoURL, m.cacheDir(videoURL))
	if err != nil {
		return nil, fmt.Errorf("failed to extract video: %w", err)
	}

	manifestRaw, _ := hls.ParseManifestPlaylist(result.ManifestRaw)

	mediaTrackInfo, err := hls.ExtractTracksFromManifest(manifestRaw)

	mediaTrackInfo.SubtitleTracks = make([]mediainfo.SubtitleTrack, 0)
	for i, subtitle := range result.Subtitles {
		track := mediainfo.SubtitleTrack{
			Index:    i,
			Language: fmt.Sprintf("%s (index: %d)", path.Base(subtitle.URL), i),
		}
		mediaTrackInfo.SubtitleTracks = append(mediaTrackInfo.SubtitleTracks, track)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to extract tracks from manifest playlist: %w", err)
	}
	return mediaTrackInfo, nil
}
