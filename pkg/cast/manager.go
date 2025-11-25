package cast

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vishen/go-chromecast/application"

	"wails-cast/pkg/extractor"
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

// StartCasting starts the casting process for the given video URL
func (m *CastManager) StartCasting(videoURL string, deviceHost string, devicePort int) error {
	proxy, err := m.prepareStream(videoURL)
	if err != nil {
		return err
	}
	defer proxy.Stop()

	proxyURL := proxy.GetProxyURL()
	fmt.Printf("\n✅ Proxy server started at %s\n", proxyURL)

	// Cast to Chromecast using custom receiver
	fmt.Printf("\n🎬 Casting to Chromecast at %s:%d...\n", deviceHost, devicePort)

	app := application.NewApplication()
	app.SetRequestTimeout(60 * time.Second)

	err = app.Start(deviceHost, devicePort)
	if err != nil {
		return fmt.Errorf("error connecting to Chromecast: %w", err)
	}
	defer app.Close(true)

	// Update to ensure receiver is ready
	if err := app.Update(); err != nil {
		return fmt.Errorf("failed to update app status: %w", err)
	}

	// Use custom receiver app ID
	customAppID := "4C4BFD9F"
	err = app.LoadApp(customAppID, proxyURL)
	if err != nil {
		return fmt.Errorf("error loading stream: %w", err)
	}

	fmt.Printf("\n✅ Successfully cast to Chromecast!\n")

	m.waitForStop()
	return nil
}

func (m *CastManager) prepareStream(videoURL string) (*stream.RemoteHLSProxy, error) {
	// 1. Calculate hash of video URL for cache key
	hash := md5.Sum([]byte(videoURL))
	cacheKey := hex.EncodeToString(hash[:])
	cacheDir := filepath.Join(m.CacheRoot, cacheKey)
	extractionFile := filepath.Join(cacheDir, "extraction.json")

	// 2. Check cache or extract
	var result *extractor.ExtractResult

	if _, err := os.Stat(extractionFile); err == nil {
		fmt.Println("Found cached extraction, loading...")
		data, err := os.ReadFile(extractionFile)
		if err == nil {
			result = &extractor.ExtractResult{}
			if err := json.Unmarshal(data, result); err != nil {
				fmt.Printf("Error unmarshaling cached extraction: %v\n", err)
				result = nil // Force re-extraction
			}
		}
	}

	if result == nil {
		fmt.Printf("Extracting video from: %s\n", videoURL)
		fmt.Println("Please click the PLAY button in the browser window...")

		var err error
		result, err = extractor.ExtractVideo(videoURL)
		if err != nil {
			return nil, fmt.Errorf("error extracting video: %w", err)
		}

		// Save extraction result
		os.MkdirAll(cacheDir, 0755)
		data, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			os.WriteFile(extractionFile, data, 0644)
		}
	}

	// 2.5 Track Selection (if master playlist)
	filteredManifestPath := filepath.Join(cacheDir, "filtered_manifest.m3u8")
	if strings.Contains(result.ManifestBody, "#EXT-X-STREAM-INF") {
		// Check if filtered manifest exists
		if _, err := os.Stat(filteredManifestPath); err == nil {
			fmt.Println("Found cached filtered manifest, using it...")
			data, err := os.ReadFile(filteredManifestPath)
			if err == nil {
				// Use the filtered manifest as the main manifest
				// The proxy will rewrite URLs on-the-fly when serving
				result.ManifestBody = string(data)
			}
		} else {
			// Interactive selection
			fmt.Println("Master playlist detected. Please select tracks:")
			newManifest, err := SelectTracksInteractive(result.ManifestBody)
			if err != nil {
				fmt.Printf("Error selecting tracks: %v\n", err)
			} else {
				// Save the filtered manifest with ORIGINAL URLs
				// The proxy will rewrite them to clean URLs when serving
				result.ManifestBody = newManifest
				os.WriteFile(filteredManifestPath, []byte(newManifest), 0644)
			}
		}
	}

	fmt.Printf("\n✅ HLS stream ready:\n")
	fmt.Printf("  URL: %s\n", result.URL)

	// Create and configure proxy
	proxy := stream.NewRemoteHLSProxy(m.LocalIP, m.ProxyPort, cacheDir)
	proxy.SetExtractor(result)

	// Start the proxy server
	err := proxy.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start proxy: %w", err)
	}

	// Get the served manifest and update the result
	servedManifest := proxy.GetServedManifest()
	result.ManifestBody = servedManifest

	// Save the served manifest for caching
	os.WriteFile(filteredManifestPath, []byte(servedManifest), 0644)

	return proxy, nil
}

func (m *CastManager) waitForStop() {
	stopCh := make(chan struct{})
	go func() {
		fmt.Println("\nPress Enter to stop...")
		fmt.Scanln()
		close(stopCh)
	}()

	<-stopCh
	fmt.Println("Stopping...")
}
