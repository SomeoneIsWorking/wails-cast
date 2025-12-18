package remote

import (
	u "net/url"
	"os"
	"path/filepath"
	"wails-cast/pkg/extractor"
	"wails-cast/pkg/filehelper"
	"wails-cast/pkg/folders"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/urlhelper"
)

type RemoteManager struct {
	items map[string]*MediaManager
	Cache bool
}

type ExtractionData struct {
	URL         string
	Cookies     map[string]string
	Headers     map[string]string
	Title       string
	ManifestURL string
}

func (m *RemoteManager) GetDownloadStatus(url string, mediaType string, track int) (*DownloadStatusQeuryResponse, error) {
	media, err := m.GetMedia(url)
	if err != nil {
		return nil, err
	}
	status, err := media.GetDownloadStatus(mediaType, track)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (m *RemoteManager) StopDownload(url string, mediaType string, index int) error {
	media, err := m.GetMedia(url)
	if err != nil {
		return err
	}
	return media.StopDownload(mediaType, index)
}

func (m *RemoteManager) StartDownload(url string, mediaType string, index int) error {
	media, err := m.GetMedia(url)
	if err != nil {
		return err
	}
	return media.StartDownload(mediaType, index)
}

// NewManager creates a new remote manager
func NewManager(cache bool) *RemoteManager {
	return &RemoteManager{
		items: make(map[string]*MediaManager),
		Cache: cache,
	}
}

func (m *RemoteManager) StopAllAndClear() error {
	for _, item := range m.items {
		err := item.StopAllAndClear()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *RemoteManager) GetMedia(url string) (*MediaManager, error) {
	parsed, err := u.Parse(url)
	if err != nil {
		return nil, err
	}
	if item, exists := m.items[url]; exists {
		return item, nil
	}
	extractionFile := extractionFile(url)
	extractionData, err := filehelper.ReadJson[ExtractionData](extractionFile)

	var manifest *hls.ManifestPlaylist
	if err != nil {
		extractionData, manifest, err = m.doExtraction(parsed)
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	if manifest == nil {
		bytes, err := os.ReadFile(hlsFile(url))
		if err != nil {
			return nil, err
		}
		manifest, err = hls.ParseManifestPlaylist(string(bytes))
		if err != nil {
			return nil, err
		}
	}

	mediaItem := NewMediaManager(
		url,
		folders.Video(url),
		extractionData.Title,
		urlhelper.Parse(extractionData.ManifestURL),
		manifest,
		&FileDownloader{
			Cookies: extractionData.Cookies,
			Headers: extractionData.Headers,
		},
		m.Cache,
	)
	m.items[url] = mediaItem
	return mediaItem, nil
}

func extractionFile(url string) string {
	return filepath.Join(folders.Video(url), "extraction.json")
}

func (*RemoteManager) doExtraction(url *u.URL) (*ExtractionData, *hls.ManifestPlaylist, error) {
	extraction, err := extractor.ExtractManifestPlaylist(url.String())
	if err != nil {
		return nil, nil, err
	}

	manifest, err := hls.ParseManifestPlaylist(extraction.Manifest)
	if err != nil {
		return nil, nil, err
	}
	hlsFile := hlsFile(url.String())
	filehelper.WriteFile(hlsFile, []byte(extraction.Manifest))

	extractionData := &ExtractionData{
		URL:         url.String(),
		Cookies:     extraction.Cookies,
		Headers:     extraction.Headers,
		Title:       extraction.Title,
		ManifestURL: url.ResolveReference(extraction.URL).String(),
	}

	filehelper.WriteJson(extractionFile(url.String()), extractionData)
	return extractionData, manifest, nil
}

func hlsFile(url string) string {
	videoFolder := folders.Video(url)
	hlsFile := filepath.Join(videoFolder, "playlist.m3u8")
	return hlsFile
}
