package hlsproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wails-cast/pkg/extractor"
)

func TestHLSProxy_Caching(t *testing.T) {
	// 1. Setup Mock Origin Server
	tsContent := []byte("fake video content")
	originServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte("#EXTM3U\n#EXTINF:10,\nsegment.ts"))
		} else if strings.HasSuffix(r.URL.Path, ".ts") {
			w.Header().Set("Content-Type", "video/mp2t")
			w.Write(tsContent)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer originServer.Close()

	// 2. Setup HLS Proxy
	result := &extractor.ExtractResult{
		BaseURL:      originServer.URL + "/",
		ManifestBody: "#EXTM3U\n#EXTINF:10,\nsegment.ts",
		Cookies:      map[string]string{},
		Headers:      map[string]string{},
	}

	proxy := NewHLSProxy(result, "127.0.0.1", 0, "")

	// Use a temp dir for cache to avoid messing with system temp
	tmpDir, err := os.MkdirTemp("", "hls-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	proxy.CacheDir = tmpDir
	proxy.ManifestPath = filepath.Join(tmpDir, "manifest.json")

	// 3. Test Playlist Request (Simulated by calling handleSegment with playlist param or just direct path if logic supports it)
	// The proxy logic for /segment?path=... handles both segments and nested playlists.
	// But the main manifest is served via handleManifest.
	// Let's test handleSegment for a nested playlist.

	req := httptest.NewRequest("GET", "/segment?path=nested.m3u8", nil)
	w := httptest.NewRecorder()

	// We need to manually trigger the logic that would happen if we requested a playlist
	// But wait, handleSegment downloads based on path.
	// If I request /segment?path=nested.m3u8, it will try to download originServer.URL + /nested.m3u8

	proxy.handleSegment(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Check manifest
	data, err := os.ReadFile(proxy.ManifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	var manifest Manifest
	err = json.Unmarshal(data, &manifest)
	if err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}

	expectedURL := originServer.URL + "/nested.m3u8"
	item, ok := manifest.Items[expectedURL]
	if !ok {
		t.Fatalf("Expected item %s in manifest", expectedURL)
	}

	if !item.IsPlaylist {
		t.Error("Expected IsPlaylist to be true")
	}

	// 4. Test Segment Request
	reqSeg := httptest.NewRequest("GET", "/segment?path=segment.ts", nil)
	wSeg := httptest.NewRecorder()

	// Note: This might try to run ffmpeg. If ffmpeg is not installed, it might fail or log error.
	// We can check if the item is added to manifest at least.
	proxy.handleSegment(wSeg, reqSeg)

	// Reload manifest
	data, _ = os.ReadFile(proxy.ManifestPath)
	json.Unmarshal(data, &manifest)

	expectedSegURL := originServer.URL + "/segment.ts"
	itemSeg, ok := manifest.Items[expectedSegURL]
	if !ok {
		t.Fatalf("Expected segment %s in manifest", expectedSegURL)
	}

	if itemSeg.IsPlaylist {
		t.Error("Expected IsPlaylist to be false for segment")
	}

	// Check if file exists in cache
	if _, err := os.Stat(itemSeg.LocalPath); os.IsNotExist(err) {
		t.Error("Expected cached file to exist")
	}
}
