package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"wails-cast/pkg/events"
	"wails-cast/pkg/extractor"
	"wails-cast/pkg/hls"
	"wails-cast/pkg/logger"
	"wails-cast/pkg/options"
	"wails-cast/pkg/subtitles"

	"github.com/pkg/errors"
)

type DownloadReport struct {
	URL       string
	MediaType string
	Track     int
	Segment   int
}

// RemoteHandler is a handler that serves HLS manifests and segments
// with captured cookies and headers
type RemoteHandler struct {
	SiteURL     string
	BaseURL     string
	Manifest    *hls.ManifestPlaylist
	ManifestRaw *hls.ManifestPlaylist
	Cookies     map[string]string
	Headers     map[string]string
	CacheDir    string // Directory for caching transcoded segments
	Options     *options.StreamOptions
	Duration    float64 // Total duration of the stream in seconds
}

type MainMap struct {
	Video []string `json:"video"`
	Audio []string `json:"audio"`
}

// NewRemoteHandler creates a new HLS handler
func NewRemoteHandler(cacheDir string, options *options.StreamOptions, result *extractor.ExtractResult) (*RemoteHandler, error) {
	os.MkdirAll(cacheDir, 0755)

	manifestRaw, err := hls.ParseManifestPlaylist(result.ManifestRaw)
	if err != nil {
		return nil, err
	}

	return &RemoteHandler{
		SiteURL:     result.SiteURL,
		CacheDir:    cacheDir,
		Options:     options,
		BaseURL:     result.BaseURL,
		Cookies:     result.Cookies,
		Headers:     result.Headers,
		Manifest:    rewriteManifestPlaylist(manifestRaw),
		ManifestRaw: manifestRaw,
	}, nil
}

func (p *RemoteHandler) GetTrackPlaylist(ctx context.Context, trackType string, index int) (hls.TrackPlaylist, error) {
	playlistContent, err := p.getTrackPlaylist(ctx, trackType, index)
	if err != nil {
		return hls.TrackPlaylist{}, err
	}
	trackPlaylist, err := hls.ParseTrackPlaylist(playlistContent)
	if err != nil {
		return hls.TrackPlaylist{}, err
	}
	return *trackPlaylist, nil
}

// ServeManifestPlaylist generates the manifest playlist
func (p *RemoteHandler) ServeManifestPlaylist(ctx context.Context) (string, error) {
	err := p.cacheMainPlaylist()
	if err != nil {
		return "", fmt.Errorf("failed to cache main playlist: %w", err)
	}

	playlist := p.Manifest.Clone()

	playlist.VideoVariants = slices.DeleteFunc(playlist.VideoVariants, func(v hls.VideoVariant) bool {
		return v.Index != p.Options.VideoTrack
	})

	videoVariant := &playlist.VideoVariants[0]
	videoVariant.Resolution = ""
	videoVariant.URI = videoVariant.URI + "?cachebust=" + time.Now().Format("20060102150405")
	if len(playlist.AudioGroups) > 0 {
		audio := &playlist.AudioGroups[videoVariant.Audio][p.Options.AudioTrack]
		audio.URI = audio.URI + "?cachebust=" + time.Now().Format("20060102150405")
		playlist.AudioGroups = map[string][]hls.AudioMedia{
			videoVariant.Audio: {*audio},
		}
	}

	// Add subtitle track if available and not burned in
	if p.Options.Subtitle.Path != "none" && !p.Options.Subtitle.BurnIn {
		playlist.SubtitleGroups = map[string][]hls.SubtitleMedia{
			"subs": {
				{
					URI:        "subtitles.vtt",
					GroupID:    "subs",
					Name:       "Subtitles",
					Language:   "en",
					Default:    true,
					Autoselect: true,
					Forced:     false,
					Index:      0,
				},
			},
		}
		videoVariant.Subtitles = "subs"
	}

	return playlist.Generate(), nil
}

// ServeTrackPlaylist generates video or audio track playlists
func (p *RemoteHandler) ServeTrackPlaylist(ctx context.Context, trackType string, trackIndex int) (string, error) {
	playlistContent, err := p.getTrackPlaylist(ctx, trackType, trackIndex)
	if err != nil {
		return "", err
	}
	playlist, err := hls.ParseTrackPlaylist(playlistContent)

	if err != nil {
		return "", err
	}

	// Add program date time tags for better sync
	addProgramDate(playlist)
	return playlist.Generate(), nil
}

func (p *RemoteHandler) cacheMainPlaylist() error {
	// Create directory
	if err := os.MkdirAll(p.CacheDir, 0755); err != nil {
		logger.Logger.Error("Failed to create manifest playlist directory", "err", err)
		return err
	}

	// 1. Save Rewritten
	rewrittenPath := filepath.Join(p.CacheDir, "playlist.m3u8")
	if err := os.WriteFile(rewrittenPath, []byte(p.Manifest.Generate()), 0644); err != nil {
		return fmt.Errorf("failed to save rewritten manifest playlist: %w", err)
	}

	// 2. Generate Map (Track Indices -> URLs)
	// We can reuse ExtractTracksFromMain logic
	mi, err := hls.ExtractTracksFromManifest(p.ManifestRaw)
	if err != nil {
		return fmt.Errorf("failed to extract tracks from manifest playlist: %w", err)
	}

	mm := MainMap{
		Video: make([]string, len(mi.VideoTracks)),
		Audio: make([]string, len(mi.AudioTracks)),
	}

	for i, t := range mi.VideoTracks {
		mm.Video[i] = hls.ResolveURL(p.BaseURL, t.URI)
	}
	for i, t := range mi.AudioTracks {
		mm.Audio[i] = hls.ResolveURL(p.BaseURL, t.URI)
	}

	// 3. Save Map
	mapPath := filepath.Join(p.CacheDir, "map.json")
	mapData, err := json.MarshalIndent(mm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal map: %w", err)
	}
	if err := os.WriteFile(mapPath, mapData, 0644); err != nil {
		return fmt.Errorf("failed to save map: %w", err)
	}
	return nil
}

// serveTrackPlaylist serves a specific video track
func (p *RemoteHandler) serveTrackPlaylist(w http.ResponseWriter, r *http.Request, trackType string, index int) error {
	playlistContent, err := p.getTrackPlaylist(r.Context(), trackType, index)
	if err != nil {
		return err
	}
	playlist, err := hls.ParseTrackPlaylist(playlistContent)

	if err != nil {
		return err
	}

	// Add program date time tags for better sync
	addProgramDate(playlist)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(playlist.Generate()))
	return nil
}

func addProgramDate(playlist *hls.TrackPlaylist) {
	baseTime := time.Now()
	cumulativeTime := 0.0

	for i := range playlist.Segments {
		segment := &playlist.Segments[i]
		segment.URI = segment.URI + "?cachebust=" + time.Now().Format("20060102150405")

		// Add program date time for each segment to help with sync
		segmentTime := baseTime.Add(time.Duration(cumulativeTime * float64(time.Second)))
		segment.ProgramDateTime = segmentTime.Format(time.RFC3339Nano)
		cumulativeTime += segment.Duration
	}
}

func (p *RemoteHandler) getTrackPlaylist(ctx context.Context, trackType string, index int) (string, error) {
	var playlistContent string
	cachedPath := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, index), "playlist.m3u8")
	_, err := os.Stat(cachedPath)
	if err == nil {
		// Load from cache
		data, err := os.ReadFile(cachedPath)
		if err != nil {
			return "", err
		}
		playlistContent = string(data)
		return playlistContent, nil
	}

	playlistContent, err = p.downloadTrackPlaylist(ctx, trackType, index)
	if err != nil {
		return "", err
	}
	return playlistContent, nil
}

func (p *RemoteHandler) downloadTrackPlaylist(ctx context.Context, trackType string, index int) (string, error) {
	targetURL, err := p.resolveTrackURL(trackType, index)
	if err != nil {
		return "", err
	}
	// Download
	resp, err := p.downloadFile(ctx, targetURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to download playlist: %s", targetURL)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	playlistContent := string(body)
	trackPlaylist, err := hls.ParseTrackPlaylist(playlistContent)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse track playlist: %s", targetURL)
	}

	rewritten, err := p.cacheTrackPlaylist(trackType, index, trackPlaylist)
	if err != nil {
		return "", err
	}
	return rewritten, nil
}

// rewriteManifestPlaylist rewrites the manifest playlist to point to local endpoints
func rewriteManifestPlaylist(playlist *hls.ManifestPlaylist) *hls.ManifestPlaylist {
	playlist = playlist.Clone()
	// Rewrite video variant URIs
	for i := range playlist.VideoVariants {
		playlist.VideoVariants[i].URI = fmt.Sprintf("/video_%d.m3u8", i)
	}

	if len(playlist.AudioGroups) > 0 {
		// Rewrite audio media URIs
		audioIdx := 0
		for groupID := range playlist.AudioGroups {
			for i := range playlist.AudioGroups[groupID] {
				if playlist.AudioGroups[groupID][i].URI != "" {
					playlist.AudioGroups[groupID][i].URI = fmt.Sprintf("/audio_%d.m3u8", audioIdx)
					audioIdx++
				}
			}
		}
	}

	return playlist
}

// cacheTrackPlaylist saves raw/rewritten playlists and a segment map
func (p *RemoteHandler) cacheTrackPlaylist(trackType string, trackIndex int, playlist *hls.TrackPlaylist) (string, error) {
	// Create directory
	dirPath := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, trackIndex))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}

	// 1. Save Raw
	rawPath := filepath.Join(dirPath, "playlist_raw.m3u8")
	if err := os.WriteFile(rawPath, []byte(playlist.Generate()), 0644); err != nil {
		logger.Logger.Error("Failed to save raw playlist", "err", err, "path", rawPath)
	}

	// 2. Rewrite and Generate Map
	rewritten, segmentMap, err := p.rewriteTrackPlaylist(trackType, trackIndex, playlist)
	if err != nil {
		return "", err
	}

	// 3. Save Map
	mapPath := filepath.Join(dirPath, "map.json")
	mapData, err := json.Marshal(segmentMap)
	if err == nil {
		if err := os.WriteFile(mapPath, mapData, 0644); err != nil {
			logger.Logger.Error("Failed to save segment map", "err", err, "path", mapPath)
		}
	} else {
		logger.Logger.Error("Failed to marshal segment map", "err", err)
	}

	// 4. Save Rewritten
	rewrittenPath := filepath.Join(dirPath, "playlist.m3u8")
	if err := os.WriteFile(rewrittenPath, []byte(rewritten), 0644); err != nil {
		logger.Logger.Error("Failed to save rewritten playlist", "err", err, "path", rewrittenPath)
	}

	return rewritten, nil
}

// rewriteTrackPlaylist rewrites a media playlist using the structured approach
func (p *RemoteHandler) rewriteTrackPlaylist(trackType string, trackIndex int, media *hls.TrackPlaylist) (string, []string, error) {
	var segmentMap []string

	// Rewrite segment URIs and build map
	for i := range media.Segments {
		segment := &media.Segments[i]

		// Resolve segment URI to absolute URL for the map
		segmentMap = append(segmentMap, p.resolveUrl(segment.URI))

		// Rewrite to local path
		segment.URI = fmt.Sprintf("/%s_%d/segment_%d.ts", trackType, trackIndex, i)
	}

	return media.Generate(), segmentMap, nil
}

func (p *RemoteHandler) resolveUrl(relativeURL string) string {
	return hls.ResolveURL(p.BaseURL, relativeURL)
}

// ServeSegment proxies segment requests with captured cookies and headers,
// and transcodes them using ffmpeg for compatibility
func (p *RemoteHandler) ServeSegment(ctx context.Context, trackType string, trackIndex int, segmentIndex int) ([]byte, error) {
	logger.Logger.Info("Proxying request", "type", trackType, "index", trackIndex, "segment", segmentIndex)

	if p.Options.NoTranscodeCache {
		rawPath, err := p.ensureSegmentExistsRaw(ctx, trackType, trackIndex, segmentIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure raw segment exists: %w", err)
		}
		return p.transcodeSegment(ctx, rawPath, "pipe:1")
	}

	transcodedPath, err := p.ensureSegmentExistsTranscoded(ctx, trackType, trackIndex, segmentIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure transcoded segment exists: %w", err)
	}
	return os.ReadFile(transcodedPath)
}

func (p *RemoteHandler) EnsureSegmentDownloaded(ctx context.Context, trackType string, trackIndex int, segmentIndex int) (string, error) {
	return p.ensureSegmentExistsRaw(ctx, trackType, trackIndex, segmentIndex)
}

func (p *RemoteHandler) ensureSegmentExistsTranscoded(ctx context.Context, trackType string, trackIndex int, segmentIndex int) (string, error) {
	transcodedPath, err := p.getSegmentPath(trackType, trackIndex, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get transcoded segment path for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}

	// Load manifest and check if segment needs regeneration
	manifest, err := hls.LoadSegmentManifest(transcodedPath + ".json")
	needsRegeneration := err != nil || !hls.ManifestMatches(manifest, p.Options, 0)

	if _, err := os.Stat(transcodedPath); err == nil && !needsRegeneration {
		return transcodedPath, nil
	}

	rawPath, err := p.ensureSegmentExistsRaw(ctx, trackType, trackIndex, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to ensure raw segment exists for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}

	_, err = p.transcodeSegment(ctx, rawPath, transcodedPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to transcode segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}
	return transcodedPath, nil
}

func (p *RemoteHandler) ensureSegmentExistsRaw(ctx context.Context, trackType string, trackIndex int, segmentIndex int) (string, error) {
	rawPath, err := p.getRawSegmentPath(trackType, trackIndex, segmentIndex)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get raw segment path for segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}
	if _, err := os.Stat(rawPath); err == nil {
		return rawPath, nil
	}
	logger.Logger.Info("Downloading segment", "type", trackType, "index", trackIndex, "segment", segmentIndex)
	startTime := time.Now()
	err = p.downloadSegment(ctx, trackType, trackIndex, segmentIndex, rawPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to download segment %d of track %s_%d", segmentIndex, trackType, trackIndex)
	}
	logger.Logger.Info("Downloaded segment", "type", trackType, "index", trackIndex, "segment", segmentIndex, "duration", time.Since(startTime).Seconds())
	return rawPath, nil
}

// resolveSegmentURL finds the absolute URL of a segment by index
func (p *RemoteHandler) resolveTrackURL(trackType string, trackIndex int) (string, error) {
	mapPath := filepath.Join(p.CacheDir, "map.json")
	if _, err := os.Stat(mapPath); os.IsNotExist(err) {
		p.cacheMainPlaylist()
	}
	mapData, err := os.ReadFile(mapPath)

	if err != nil {
		return "", err
	}
	var mainMap MainMap
	err = json.Unmarshal(mapData, &mainMap)
	if err != nil {
		return "", err
	}

	switch trackType {
	case "audio":
		if trackIndex < 0 || trackIndex >= len(mainMap.Audio) {
			return "", fmt.Errorf("audio track index %d out of range (map size: %d)", trackIndex, len(mainMap.Audio))
		}
		return mainMap.Audio[trackIndex], nil
	case "video":
		if trackIndex < 0 || trackIndex >= len(mainMap.Video) {
			return "", fmt.Errorf("video track index %d out of range (map size: %d)", trackIndex, len(mainMap.Video))
		}
		return mainMap.Video[trackIndex], nil
	}
	return "", fmt.Errorf("unknown track type: %s", trackType)
}

// resolveSegmentURL finds the absolute URL of a segment by index
func (p *RemoteHandler) resolveSegmentURL(trackType string, trackIndex int, segmentIndex int) (string, error) {
	trackDir := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, trackIndex))
	mapPath := filepath.Join(trackDir, "map.json")
	mapData, err := os.ReadFile(mapPath)
	if err != nil {
		return "", err
	}
	var segmentMap []string
	err = json.Unmarshal(mapData, &segmentMap)
	if err != nil {
		return "", err
	}
	if segmentIndex < 0 || segmentIndex >= len(segmentMap) {
		return "", fmt.Errorf("segment index %d out of range (map size: %d)", segmentIndex, len(segmentMap))
	}
	return segmentMap[segmentIndex], nil
}

func (p *RemoteHandler) downloadSegment(ctx context.Context, trackType string, trackIndex int, segmentIndex int, rawPath string) error {
	url, err := p.resolveSegmentURL(trackType, trackIndex, segmentIndex)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve segment URL for trackType=%s, trackIndex=%d, segmentIndex=%d", trackType, trackIndex, segmentIndex)
	}
	resp, err := p.downloadFile(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to download segment: %s", url)
	}
	defer resp.Body.Close()

	// Save to file
	rawFile, err := os.Create(rawPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create raw segment file: %s", rawPath)
	}
	defer rawFile.Close()

	_, err = io.Copy(rawFile, resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to write to raw segment file: %s", rawPath)
	}

	events.Download.Emit("download:progress", DownloadReport{
		URL:       p.SiteURL,
		MediaType: trackType,
		Track:     trackIndex,
		Segment:   segmentIndex,
	})

	return nil
}

func (p *RemoteHandler) transcodeSegment(ctx context.Context, rawPath string, transcodedPath string) ([]byte, error) {
	startTime := time.Now()

	subtitle := ""

	if p.Options.Subtitle.BurnIn {
		subtitle = p.Options.Subtitle.Path
		if index, found := strings.CutPrefix(subtitle, "embedded:"); found {
			// For remote streams, embedded just means found on website, in this case, we use the cached file
			subtitle = fmt.Sprintf("external:%s", filepath.Join(p.CacheDir, fmt.Sprintf("subtitle_%s.vtt", index)))
		}
	}

	opts := &hls.TranscodeOptions{
		InputPath:      rawPath,
		OutputPath:     transcodedPath,
		StartTime:      0,
		Duration:       0,
		Subtitle:       subtitle,
		Bitrate:        p.Options.Bitrate,
		FontSize:       p.Options.Subtitle.FontSize,
		MaxOutputWidth: p.Options.MaxOutputWidth,
	}
	buffer, err := hls.TranscodeSegment(ctx, opts)
	if err != nil {
		return nil, err
	}
	logger.Logger.Info("Transcoded segment", "input", rawPath, "output", transcodedPath, "duration", time.Since(startTime).Seconds())
	if transcodedPath != "pipe:1" {
		err = opts.Save(transcodedPath + ".json")
	}
	return buffer, err
}

func (p *RemoteHandler) getSegmentPath(trackType string, trackIndex int, segmentIndex int) (string, error) {
	trackDir, err := p.getTrackDir(trackType, trackIndex)
	if err != nil {
		return "", err
	}

	localPath := filepath.Join(trackDir, fmt.Sprintf("segment_%d.ts", segmentIndex))
	return localPath, nil
}

func (p *RemoteHandler) getRawSegmentPath(trackType string, trackIndex int, segmentIndex int) (string, error) {
	trackDir, err := p.getTrackDir(trackType, trackIndex)
	if err != nil {
		return "", err
	}

	localPath := filepath.Join(trackDir, fmt.Sprintf("segment_%d_raw.ts", segmentIndex))
	return localPath, nil
}

func (p *RemoteHandler) getTrackDir(trackType string, trackIndex int) (string, error) {
	trackDir := filepath.Join(p.CacheDir, fmt.Sprintf("%s_%d", trackType, trackIndex))
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		return "", err
	}
	return trackDir, nil
}

// serveFile serves a local file
func (p *RemoteHandler) serveFile(w http.ResponseWriter, path string, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ServeSubtitles returns the subtitle file in WebVTT format
func (p *RemoteHandler) ServeSubtitles(ctx context.Context) (string, error) {
	if p.Options.Subtitle.Path == "none" || p.Options.Subtitle.BurnIn {
		return "", fmt.Errorf("no external subtitles available")
	}

	subtitlePath := p.Options.Subtitle.Path

	// Handle embedded subtitles - they're cached in the cache directory
	if index, found := strings.CutPrefix(subtitlePath, "embedded:"); found {
		subtitlePath = filepath.Join(p.CacheDir, fmt.Sprintf("subtitle_%s.vtt", index))
	} else if path, found := strings.CutPrefix(subtitlePath, "external:"); found {
		subtitlePath = path
	}
	// Otherwise it's a direct path, use as-is

	// Read subtitle file
	data, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Check if it's already WebVTT
	content := string(data)
	if strings.HasPrefix(content, "WEBVTT") {
		return content, nil
	}

	// Otherwise parse and convert to WebVTT
	subtitles, err := subtitles.Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse subtitle file: %w", err)
	}

	return subtitles.ToWebVTTString(), nil
}

// downloadFile downloads a file with cookies and headers
func (p *RemoteHandler) downloadFile(ctx context.Context, url string) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range p.Headers {
		req.Header.Set(key, value)
	}

	if len(p.Cookies) > 0 {
		var cookieParts []string
		for key, value := range p.Cookies {
			cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", key, value))
		}
		req.Header.Set("Cookie", strings.Join(cookieParts, "; "))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	return resp, nil
}
