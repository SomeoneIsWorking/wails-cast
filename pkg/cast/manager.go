package cast

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vishen/go-chromecast/application"

	"wails-cast/pkg/extractor"
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

// ChromecastApp wraps the chromecast application
type ChromecastApp struct {
	App  *application.Application
	Host string
	Port int
}

// NewCastManager creates a new CastManager
func NewCastManager(localIP string, proxyPort int) *CastManager {
	cacheRoot := filepath.Join(os.TempDir(), "wails-cast-cache")
	os.MkdirAll(cacheRoot, 0755)
	return &CastManager{
		LocalIP:   localIP,
		ProxyPort: proxyPort,
		CacheRoot: cacheRoot,
	}
}

func (m *CastManager) CreateRemoteHandler(videoURL string, options options.StreamOptions) (*stream.RemoteHandler, error) {
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

func getExtractionJson(videoURL string, cacheDir string) (*extractor.ExtractResult, error) {
	extractionFile := filepath.Join(cacheDir, "extraction.json")

	var result *extractor.ExtractResult

	if _, err := os.Stat(extractionFile); err == nil {
		logger.Logger.Info("Found cached extraction, loading...")
		data, err := os.ReadFile(extractionFile)
		if err == nil {
			result = &extractor.ExtractResult{}
			if err := json.Unmarshal(data, result); err != nil {
				fmt.Printf("Error unmarshaling cached extraction: %v\n", err)
				result = nil
			}
		}
	}

	if result != nil {
		return result, nil
	}
	fmt.Printf("Extracting video from: %s\n", videoURL)
	fmt.Println("Please click the PLAY button in the browser window...")

	var err error
	result, err = extractor.ExtractManifestPlaylist(videoURL)
	if err != nil {
		return nil, fmt.Errorf("error extracting video: %w", err)
	}

	// Save extraction result
	os.MkdirAll(cacheDir, 0755)
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

	if err != nil {
		return nil, fmt.Errorf("failed to extract tracks from manifest playlist: %w", err)
	}
	return mediaTrackInfo, nil
}
