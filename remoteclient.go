package main

// Remote-client methods let this desktop instance act as a controller for
// *another* wails-cast instance (e.g. the fedora host) over its remote HTTP API.
// They mirror the mobile companion app: discover instances on the LAN, browse
// the remote library, list cast targets, start/stop playback, and drive the
// qBittorrent magnet workflow — all by calling the endpoints defined in
// httpserver.go on the remote host.
//
// All HTTP work happens here in Go (rather than the webview) so the auth token
// stays server-side, response types are reused, and mDNS discovery is possible.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"

	"wails-cast/pkg/options"
)

// CastInstance is a wails-cast instance discovered on the LAN over mDNS.
type CastInstance struct {
	Name string `json:"name"`
	Host string `json:"host"` // IPv4 or hostname
	Port int    `json:"port"`
	URL  string `json:"url"` // convenience: http://host:port
}

// RemoteDevice is a cast target reported by a remote instance's /devices.
type RemoteDevice struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
	UUID string `json:"uuid"`
}

// RemotePlayOptions mirrors the optional track/subtitle/quality selection
// accepted by the remote /play and /play-url endpoints.
type RemotePlayOptions struct {
	VideoTrack   int     `json:"videoTrack"`
	AudioTrack   int     `json:"audioTrack"`
	SubtitlePath string  `json:"subtitlePath"`
	Quality      *string `json:"quality"`
}

// remoteHTTP is the shared HTTP client for talking to remote instances.
var remoteHTTP = &http.Client{Timeout: 12 * time.Second}

// remoteRequest performs a JSON request against base+path on a remote instance,
// attaching the auth token, and decodes the response into out (may be nil).
func remoteRequest(method, base, path, token string, body any, out any) error {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return fmt.Errorf("remote base URL is required")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}

	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Cast-Token", token)
	}

	resp, err := remoteHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to surface the remote error envelope.
		var er errorResponse
		if json.NewDecoder(resp.Body).Decode(&er) == nil && er.Error != "" {
			return fmt.Errorf("remote: %s", er.Error)
		}
		return fmt.Errorf("remote: status %s", resp.Status)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("remote: decode response: %w", err)
		}
	}
	return nil
}

// DiscoverCastInstances browses the LAN for wails-cast instances advertising the
// _wailscast._tcp service and returns them. Blocks up to ~3s.
func (a *App) DiscoverCastInstances() ([]CastInstance, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := resolver.Browse(ctx, mdnsService, "local.", entries); err != nil {
		return nil, fmt.Errorf("mDNS browse: %w", err)
	}

	// Collect this machine's own IPs so we can skip our own advertisement —
	// the controller shouldn't list itself as a remote instance.
	localIPs := localIPSet()

	found := make([]CastInstance, 0)
	seen := map[string]bool{}
	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return found, nil
			}
			if entry == nil {
				continue
			}
			// Skip self: any advertisement resolving to one of our own IPs.
			if entryIsSelf(entry, localIPs) {
				continue
			}
			host := ""
			if len(entry.AddrIPv4) > 0 {
				host = entry.AddrIPv4[0].String()
			} else if entry.HostName != "" {
				host = strings.TrimSuffix(entry.HostName, ".")
			}
			if host == "" {
				continue
			}
			key := fmt.Sprintf("%s:%d", host, entry.Port)
			if seen[key] {
				continue
			}
			seen[key] = true
			name := entry.Instance
			if name == "" {
				name = host
			}
			found = append(found, CastInstance{
				Name: name,
				Host: host,
				Port: entry.Port,
				URL:  fmt.Sprintf("http://%s:%d", host, entry.Port),
			})
		case <-ctx.Done():
			return found, nil
		}
	}
}

// localIPSet returns the set of this machine's non-loopback IPv4/IPv6 addresses.
func localIPSet() map[string]bool {
	set := map[string]bool{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return set
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok {
			set[ipNet.IP.String()] = true
		}
	}
	return set
}

// entryIsSelf reports whether an mDNS entry resolves to one of our own IPs.
func entryIsSelf(entry *zeroconf.ServiceEntry, localIPs map[string]bool) bool {
	for _, ip := range entry.AddrIPv4 {
		if localIPs[ip.String()] {
			return true
		}
	}
	for _, ip := range entry.AddrIPv6 {
		if localIPs[ip.String()] {
			return true
		}
	}
	return false
}

// RemotePing checks connectivity to a remote instance.
func (a *App) RemotePing(base, token string) (bool, error) {
	var resp pingResponse
	if err := remoteRequest(http.MethodGet, base, "/ping", token, nil, &resp); err != nil {
		return false, err
	}
	return resp.OK, nil
}

// RemoteLibrary lists the remote instance's library items.
func (a *App) RemoteLibrary(base, token string) ([]LibraryItem, error) {
	var resp libraryResponse
	if err := remoteRequest(http.MethodGet, base, "/library", token, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Items == nil {
		resp.Items = []LibraryItem{}
	}
	return resp.Items, nil
}

// RemoteDevices lists the cast targets available to the remote instance.
func (a *App) RemoteDevices(base, token string) ([]RemoteDevice, error) {
	var resp struct {
		Items []RemoteDevice `json:"items"`
	}
	if err := remoteRequest(http.MethodGet, base, "/devices", token, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Items == nil {
		resp.Items = []RemoteDevice{}
	}
	return resp.Items, nil
}

// RemoteState returns the remote instance's current playback state.
func (a *App) RemoteState(base, token string) (*PlaybackState, error) {
	var state PlaybackState
	if err := remoteRequest(http.MethodGet, base, "/state", token, nil, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// RemotePlay starts playback of a library item on the remote instance.
// deviceIp is the target ("local" for the remote desktop, or a Chromecast host).
func (a *App) RemotePlay(base, token, id, deviceIp string, opts RemotePlayOptions) (*PlaybackState, error) {
	body := map[string]any{
		"id":           id,
		"deviceIp":     deviceIp,
		"videoTrack":   opts.VideoTrack,
		"audioTrack":   opts.AudioTrack,
		"subtitlePath": opts.SubtitlePath,
		"quality":      opts.Quality,
	}
	var resp playResponse
	if err := remoteRequest(http.MethodPost, base, "/play", token, body, &resp); err != nil {
		return nil, err
	}
	return &resp.State, nil
}

// RemoteControl applies a transport command (pause/resume/stop/seek/volume/mute)
// to the remote instance's active playback.
func (a *App) RemoteControl(base, token, action string, value float64) (*PlaybackState, error) {
	body := map[string]any{"action": action, "value": value}
	var resp playResponse
	if err := remoteRequest(http.MethodPost, base, "/control", token, body, &resp); err != nil {
		return nil, err
	}
	return &resp.State, nil
}

// RemoteTrackInfo fetches the track/subtitle picker info for a remote item.
func (a *App) RemoteTrackInfo(base, token, id string) (*TrackDisplayInfo, error) {
	var info TrackDisplayInfo
	path := "/track-info?id=" + url.QueryEscape(id)
	if err := remoteRequest(http.MethodGet, base, path, token, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// RemoteAddTorrent sends a magnet link to the remote instance's qBittorrent.
func (a *App) RemoteAddTorrent(base, token, magnet string) error {
	body := map[string]any{"magnet": magnet}
	return remoteRequest(http.MethodPost, base, "/torrent/add", token, body, nil)
}

// RemoteTorrents lists the remote instance's qBittorrent torrents with progress.
func (a *App) RemoteTorrents(base, token string) ([]TorrentStatus, error) {
	var resp struct {
		Items []TorrentStatus `json:"items"`
	}
	if err := remoteRequest(http.MethodGet, base, "/torrents", token, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Items == nil {
		resp.Items = []TorrentStatus{}
	}
	return resp.Items, nil
}

// ----------------------------------------------------------------------------
// Library management on a remote instance (tree / identify / organize / translate)
// ----------------------------------------------------------------------------

// RemoteLibraryTree fetches the remote instance's full Show/Season/Episode tree.
func (a *App) RemoteLibraryTree(base, token string) (*LibraryScanResult, error) {
	var result LibraryScanResult
	if err := remoteRequest(http.MethodGet, base, "/library/tree", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RemoteIdentify enriches a scan result with TMDB metadata on the remote.
func (a *App) RemoteIdentify(base, token string, result LibraryScanResult) (*LibraryScanResult, error) {
	var out LibraryScanResult
	if err := remoteRequest(http.MethodPost, base, "/library/identify", token, result, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoteOrganizePreview builds the organize move plan on the remote.
func (a *App) RemoteOrganizePreview(base, token string, result LibraryScanResult) ([]OrganizeMove, error) {
	var resp struct {
		Plan []OrganizeMove `json:"plan"`
	}
	if err := remoteRequest(http.MethodPost, base, "/library/organize/preview", token, result, &resp); err != nil {
		return nil, err
	}
	if resp.Plan == nil {
		resp.Plan = []OrganizeMove{}
	}
	return resp.Plan, nil
}

// RemoteOrganizeExecute runs an organize plan on the remote.
func (a *App) RemoteOrganizeExecute(base, token string, plan []OrganizeMove) error {
	return remoteRequest(http.MethodPost, base, "/library/organize/execute", token, map[string]any{"plan": plan}, nil)
}

// RemoteTranslateSeason starts a season-batch translation on the remote.
func (a *App) RemoteTranslateSeason(base, token, showName, seasonName string, episodePaths []string, language string) error {
	body := map[string]any{
		"showName":     showName,
		"seasonName":   seasonName,
		"episodePaths": episodePaths,
		"language":     language,
	}
	return remoteRequest(http.MethodPost, base, "/library/translate-season", token, body, nil)
}

// RemoteSeasonStatus polls season-translation progress on the remote.
func (a *App) RemoteSeasonStatus(base, token string) (*SeasonTranslateProgress, error) {
	var status SeasonTranslateProgress
	if err := remoteRequest(http.MethodGet, base, "/library/translate-season/status", token, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// RemoteSeasonCancel cancels a season-batch translation on the remote.
func (a *App) RemoteSeasonCancel(base, token string) error {
	return remoteRequest(http.MethodPost, base, "/library/translate-season/cancel", token, nil, nil)
}

// RemoteTranslateFile starts a single-file translation on the remote.
func (a *App) RemoteTranslateFile(base, token, id, language string) error {
	body := map[string]any{"id": id, "language": language}
	return remoteRequest(http.MethodPost, base, "/translate", token, body, nil)
}

// RemoteTranslateStatus polls single-file translation progress on the remote.
func (a *App) RemoteTranslateStatus(base, token string) (*translateStatus, error) {
	var status translateStatus
	if err := remoteRequest(http.MethodGet, base, "/translate-status", token, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// RemoteUpdateSubtitle applies live subtitle settings (size/sync/style) to the
// remote's active playback.
func (a *App) RemoteUpdateSubtitle(base, token string, opts options.SubtitleCastOptions) error {
	return remoteRequest(http.MethodPost, base, "/subtitle", token, opts, nil)
}
