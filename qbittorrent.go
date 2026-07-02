package main

// Minimal qBittorrent Web API (v2) client used by the remote API's /torrent/*
// endpoints. It supports just what wails-cast needs: log in, add a magnet link,
// and list torrents with their download progress.
//
// The client keeps a session cookie (SID) and transparently re-authenticates
// when qBittorrent rejects a request with 403 Forbidden (expired session).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"wails-cast/pkg/castapi"
)

// TorrentStatus is the subset of a qBittorrent torrent we expose to clients.
type TorrentStatus = castapi.TorrentStatus

// qbtClient is a session-aware qBittorrent Web API client.
type qbtClient struct {
	baseURL string
	user    string
	pass    string
	http    *http.Client

	mu       sync.Mutex
	loggedIn bool
}

// newQbtClient builds a client from the given credentials. baseURL must be a
// non-empty Web UI URL (e.g. "http://127.0.0.1:8080").
func newQbtClient(baseURL, user, pass string) (*qbtClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("qBittorrent URL is not configured")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &qbtClient{
		baseURL: baseURL,
		user:    user,
		pass:    pass,
		http:    &http.Client{Jar: jar, Timeout: 15 * time.Second},
	}, nil
}

// login authenticates against /api/v2/auth/login and stores the session cookie
// in the client's cookie jar.
func (c *qbtClient) login() error {
	form := url.Values{}
	form.Set("username", c.user)
	form.Set("password", c.pass)

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// qBittorrent validates the Referer/Origin against its host header.
	req.Header.Set("Referer", c.baseURL)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("qBittorrent login: %w", err)
	}
	defer resp.Body.Close()

	// qBittorrent replies 200 "Ok." on most builds but 204 No Content on others;
	// accept any 2xx as success and rely on the body / cookie below.
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("qBittorrent login failed: status %s", resp.Status)
	}
	// On bad credentials qBittorrent returns 200 with body "Fails.".
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	if strings.Contains(buf.String(), "Fails") {
		return fmt.Errorf("qBittorrent login failed: invalid username or password")
	}

	c.mu.Lock()
	c.loggedIn = true
	c.mu.Unlock()
	return nil
}

// ensureLogin logs in if we have not yet established a session.
func (c *qbtClient) ensureLogin() error {
	c.mu.Lock()
	in := c.loggedIn
	c.mu.Unlock()
	if in {
		return nil
	}
	return c.login()
}

// do performs an authenticated request, re-logging in once on a 403 response.
// build is called to (re)construct the request so the body can be replayed.
func (c *qbtClient) do(build func() (*http.Request, error)) (*http.Response, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}

	req, err := build()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", c.baseURL)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		// Session likely expired — re-authenticate and retry once.
		resp.Body.Close()
		c.mu.Lock()
		c.loggedIn = false
		c.mu.Unlock()
		if err := c.login(); err != nil {
			return nil, err
		}
		req, err = build()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Referer", c.baseURL)
		return c.http.Do(req)
	}
	return resp, nil
}

// AddMagnet adds a magnet link. savePath is the download directory; when empty
// qBittorrent uses its own default save path.
func (c *qbtClient) AddMagnet(magnet, savePath string) error {
	magnet = strings.TrimSpace(magnet)
	if magnet == "" {
		return fmt.Errorf("magnet link is required")
	}

	build := func() (*http.Request, error) {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("urls", magnet)
		if savePath != "" {
			_ = mw.WriteField("savepath", savePath)
			_ = mw.WriteField("autoTMM", "false")
		}
		if err := mw.Close(); err != nil {
			return nil, err
		}
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v2/torrents/add", &body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		return req, nil
	}

	resp, err := c.do(build)
	if err != nil {
		return fmt.Errorf("qBittorrent add magnet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("qBittorrent add magnet failed: status %s", resp.Status)
	}
	// qBittorrent replies "Ok." on success, "Fails." when the magnet is invalid.
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	if strings.Contains(buf.String(), "Fails") {
		return fmt.Errorf("qBittorrent rejected the magnet link")
	}
	return nil
}

// Torrents returns the current torrent list with progress.
func (c *qbtClient) Torrents() ([]TorrentStatus, error) {
	build := func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, c.baseURL+"/api/v2/torrents/info", nil)
	}

	resp, err := c.do(build)
	if err != nil {
		return nil, fmt.Errorf("qBittorrent list torrents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("qBittorrent list torrents failed: status %s", resp.Status)
	}

	var raw []TorrentStatus
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("qBittorrent list torrents: decode: %w", err)
	}
	if raw == nil {
		raw = []TorrentStatus{}
	}
	return raw, nil
}
