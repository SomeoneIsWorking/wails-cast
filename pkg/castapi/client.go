// Package remote provides a LAN HTTP client and mDNS discovery for talking to
// a wails-cast instance's remote API. It is shared by the desktop app (to act
// as a controller for another instance) and the Fyne mobile companion app.
package castapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wails-cast/pkg/options"
)

type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
}

func New(base, token string) *Client {
	return &Client{
		Base:  base,
		Token: token,
		HTTP:  &http.Client{Timeout: 12 * time.Second},
	}
}

func (c *Client) do(method, path string, body, out any) error {
	base := strings.TrimRight(strings.TrimSpace(c.Base), "/")
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
	if c.Token != "" {
		req.Header.Set("X-Cast-Token", c.Token)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er ErrorResponse
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

func (c *Client) Ping() (bool, error) {
	var resp PingResponse
	if err := c.do(http.MethodGet, "/ping", nil, &resp); err != nil {
		return false, err
	}
	return resp.OK, nil
}

func (c *Client) Library() ([]LibraryItem, error) {
	var resp LibraryResponse
	if err := c.do(http.MethodGet, "/library", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Items == nil {
		resp.Items = []LibraryItem{}
	}
	return resp.Items, nil
}

func (c *Client) Devices() ([]RemoteDevice, error) {
	var resp struct {
		Items []RemoteDevice `json:"items"`
	}
	if err := c.do(http.MethodGet, "/devices", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Items == nil {
		resp.Items = []RemoteDevice{}
	}
	return resp.Items, nil
}

func (c *Client) State() (*PlaybackState, error) {
	var state PlaybackState
	if err := c.do(http.MethodGet, "/state", nil, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *Client) Play(id, deviceIp string, opts PlayOptions) (*PlaybackState, error) {
	body := map[string]any{
		"id":           id,
		"deviceIp":     deviceIp,
		"videoTrack":   opts.VideoTrack,
		"audioTrack":   opts.AudioTrack,
		"subtitlePath": opts.SubtitlePath,
		"quality":      opts.Quality,
	}
	var resp PlayResponse
	if err := c.do(http.MethodPost, "/play", body, &resp); err != nil {
		return nil, err
	}
	return &resp.State, nil
}

func (c *Client) Control(action string, value float64) (*PlaybackState, error) {
	body := map[string]any{"action": action, "value": value}
	var resp PlayResponse
	if err := c.do(http.MethodPost, "/control", body, &resp); err != nil {
		return nil, err
	}
	return &resp.State, nil
}

func (c *Client) TrackInfo(id string) (*TrackDisplayInfo, error) {
	var info TrackDisplayInfo
	path := "/track-info?id=" + url.QueryEscape(id)
	if err := c.do(http.MethodGet, path, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) AddTorrent(magnet string) error {
	return c.do(http.MethodPost, "/torrent/add", map[string]any{"magnet": magnet}, nil)
}

func (c *Client) Torrents() ([]TorrentStatus, error) {
	var resp struct {
		Items []TorrentStatus `json:"items"`
	}
	if err := c.do(http.MethodGet, "/torrents", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Items == nil {
		resp.Items = []TorrentStatus{}
	}
	return resp.Items, nil
}

func (c *Client) LibraryTree() (*LibraryScanResult, error) {
	var result LibraryScanResult
	if err := c.do(http.MethodGet, "/library/tree", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) Identify(result LibraryScanResult) (*LibraryScanResult, error) {
	var out LibraryScanResult
	if err := c.do(http.MethodPost, "/library/identify", result, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) OrganizePreview(result LibraryScanResult) ([]OrganizeMove, error) {
	var resp struct {
		Plan []OrganizeMove `json:"plan"`
	}
	if err := c.do(http.MethodPost, "/library/organize/preview", result, &resp); err != nil {
		return nil, err
	}
	if resp.Plan == nil {
		resp.Plan = []OrganizeMove{}
	}
	return resp.Plan, nil
}

func (c *Client) OrganizeExecute(plan []OrganizeMove) error {
	return c.do(http.MethodPost, "/library/organize/execute", map[string]any{"plan": plan}, nil)
}

func (c *Client) TranslateSeason(showName, seasonName string, episodePaths []string, language string) error {
	body := map[string]any{
		"showName":     showName,
		"seasonName":   seasonName,
		"episodePaths": episodePaths,
		"language":     language,
	}
	return c.do(http.MethodPost, "/library/translate-season", body, nil)
}

func (c *Client) SeasonStatus() (*SeasonTranslateProgress, error) {
	var status SeasonTranslateProgress
	if err := c.do(http.MethodGet, "/library/translate-season/status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) SeasonCancel() error {
	return c.do(http.MethodPost, "/library/translate-season/cancel", nil, nil)
}

func (c *Client) TranslateFile(id, language string) error {
	return c.do(http.MethodPost, "/translate", map[string]any{"id": id, "language": language}, nil)
}

func (c *Client) TranslateStatus() (*TranslateStatus, error) {
	var status TranslateStatus
	if err := c.do(http.MethodGet, "/translate-status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) UpdateSubtitle(opts options.SubtitleCastOptions) error {
	return c.do(http.MethodPost, "/subtitle", opts, nil)
}
