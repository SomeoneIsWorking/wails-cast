package main

// HTTPServer exposes a small REST/JSON API so remote clients (e.g. an Android
// companion app) can browse the media library, trigger playback of a library
// item, and send an arbitrary URL to be played on this desktop.
//
// Endpoints
//   GET  /ping          – health / discovery
//   GET  /library       – list library items (delegates to LibraryLister)
//   POST /play          – play a library item by id
//   POST /play-url      – play an arbitrary URL
//
// The server is opt-in: it only starts when Settings.RemoteAPIEnabled is true.
// It binds to 0.0.0.0 so devices on the same LAN can reach it.
//
// Authentication: if Settings.RemoteAPIToken is non-empty every request must
// carry the header  X-Cast-Token: <token>.  Leave the token blank to allow
// unauthenticated access (suitable for trusted home networks).
//
// CORS: the server adds permissive CORS headers.  Native Android apps do not
// use the browser's CORS mechanism, but this makes the API usable from a web
// client too.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"

	"wails-cast/pkg/options"
)

// mdnsService is the mDNS/Bonjour service type the desktop advertises so the
// companion app can discover it automatically on the LAN.
const mdnsService = "_wailscast._tcp"

// ----------------------------------------------------------------------------
// Library abstraction (consumed by /library)
// ----------------------------------------------------------------------------

// LibraryItem is the shape returned to remote clients for a single media item.
type LibraryItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Duration int    `json:"duration,omitempty"` // seconds, 0 if unknown
}

// LibraryLister is the interface the HTTP server uses to list library items.
// The Library feature (being built in a parallel worktree) should implement
// this interface and register it via HTTPServer.SetLibraryLister().
// Until that feature lands the server falls back to listing playback history.
type LibraryLister interface {
	ListLibraryItems() ([]LibraryItem, error)
}

// ----------------------------------------------------------------------------
// Request / response types
// ----------------------------------------------------------------------------

// pingResponse is returned by GET /ping.
type pingResponse struct {
	OK        bool   `json:"ok"`
	AppName   string `json:"app"`
	Timestamp string `json:"timestamp"`
}

// libraryResponse wraps the item list so we can add pagination later.
type libraryResponse struct {
	Items []LibraryItem `json:"items"`
}

// playRequest is the body for POST /play.
type playRequest struct {
	ID       string `json:"id"`       // library item id (== path)
	DeviceIP string `json:"deviceIp"` // target device; "local" for desktop-only
}

// playURLRequest is the body for POST /play-url.
type playURLRequest struct {
	URL      string `json:"url"`
	DeviceIP string `json:"deviceIp"` // target device; "local" for desktop-only
}

// playResponse is returned after a successful play command.
type playResponse struct {
	OK    bool         `json:"ok"`
	State PlaybackState `json:"state"`
}

// errorResponse is the standard error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// ----------------------------------------------------------------------------
// HTTPServer
// ----------------------------------------------------------------------------

// HTTPServer is the remote-control API server embedded in the desktop app.
type HTTPServer struct {
	app      *App
	listener net.Listener
	srv      *http.Server
	lister   LibraryLister
	mdns     *zeroconf.Server
	mu       sync.Mutex
	running  bool
}

// NewHTTPServer creates an HTTPServer.  Call Start() to listen.
func NewHTTPServer(app *App) *HTTPServer {
	return &HTTPServer{app: app}
}

// SetLibraryLister registers an implementation of LibraryLister.
// This is called by the Library feature once it is initialised.
// Safe to call at any time; replaces any previously registered lister.
func (h *HTTPServer) SetLibraryLister(l LibraryLister) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lister = l
}

// Start begins accepting connections on the configured port.
// It is safe to call Start on an already-running server (no-op).
func (h *HTTPServer) Start(port int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return nil
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("remote API: listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", h.handlePing)
	mux.HandleFunc("/library", h.handleLibrary)
	mux.HandleFunc("/play", h.handlePlay)
	mux.HandleFunc("/play-url", h.handlePlayURL)

	h.listener = ln
	h.srv = &http.Server{
		Handler:      h.withMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	h.running = true

	go func() {
		logger.Info("Remote API listening", "addr", addr)
		if err := h.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("Remote API server error", "error", err)
		}
	}()

	// Advertise over mDNS so the companion app can auto-discover this host.
	instance, _ := os.Hostname()
	if instance == "" {
		instance = "wails-cast"
	}
	if mdns, err := zeroconf.Register(instance, mdnsService, "local.", port, []string{"app=wails-cast"}, nil); err != nil {
		logger.Warn("Remote API: mDNS advertisement failed", "error", err)
	} else {
		h.mdns = mdns
		logger.Info("Remote API mDNS advertised", "instance", instance, "service", mdnsService, "port", port)
	}

	return nil
}

// Stop shuts the server down gracefully.
func (h *HTTPServer) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}
	h.running = false

	if h.mdns != nil {
		h.mdns.Shutdown()
		h.mdns = nil
	}
	if h.srv != nil {
		_ = h.srv.Close()
		h.srv = nil
	}
	if h.listener != nil {
		_ = h.listener.Close()
		h.listener = nil
	}
	logger.Info("Remote API stopped")
}

// IsRunning reports whether the server is currently running.
func (h *HTTPServer) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// ----------------------------------------------------------------------------
// Middleware
// ----------------------------------------------------------------------------

func (h *HTTPServer) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS – permissive for LAN use; native Android clients ignore this
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Cast-Token")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Token auth (optional)
		settings := h.app.settingsStore.Get()
		if settings.RemoteAPIToken != "" {
			token := r.Header.Get("X-Cast-Token")
			if token != settings.RemoteAPIToken {
				writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid or missing X-Cast-Token"})
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// ----------------------------------------------------------------------------
// Handlers
// ----------------------------------------------------------------------------

func (h *HTTPServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, pingResponse{
		OK:        true,
		AppName:   "wails-cast",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HTTPServer) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	h.mu.Lock()
	lister := h.lister
	h.mu.Unlock()

	var items []LibraryItem
	var err error

	if lister != nil {
		items, err = lister.ListLibraryItems()
		if err != nil {
			logger.Error("Remote API: library lister error", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list library"})
			return
		}
	} else {
		// Degrade gracefully: return playback history as a library proxy
		items = h.historyAsLibraryItems()
	}

	if items == nil {
		items = []LibraryItem{}
	}
	writeJSON(w, http.StatusOK, libraryResponse{Items: items})
}

func (h *HTTPServer) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req playRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id is required"})
		return
	}

	deviceIP := req.DeviceIP
	if deviceIP == "" {
		deviceIP = "local"
	}

	state, err := h.app.CastToDevice(deviceIP, req.ID, &options.CastOptions{
		SubtitlePath: "none",
		VideoTrack:   0,
		AudioTrack:   0,
		Bitrate:      h.app.settingsStore.Get().DefaultQuality,
	})
	if err != nil {
		logger.Error("Remote API: play failed", "id", req.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, playResponse{OK: true, State: *state})
}

func (h *HTTPServer) handlePlayURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req playURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url is required"})
		return
	}

	deviceIP := req.DeviceIP
	if deviceIP == "" {
		deviceIP = "local"
	}

	state, err := h.app.CastToDevice(deviceIP, req.URL, &options.CastOptions{
		SubtitlePath: "none",
		VideoTrack:   0,
		AudioTrack:   0,
		Bitrate:      h.app.settingsStore.Get().DefaultQuality,
	})
	if err != nil {
		logger.Error("Remote API: play-url failed", "url", req.URL, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, playResponse{OK: true, State: *state})
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// historyAsLibraryItems converts the playback history into LibraryItems.
// Used as a fallback when no LibraryLister is registered.
func (h *HTTPServer) historyAsLibraryItems() []LibraryItem {
	history := h.app.historyStore.GetAll()
	items := make([]LibraryItem, 0, len(history))
	for _, hi := range history {
		items = append(items, LibraryItem{
			ID:   hi.FileNameOrUrl,
			Name: hi.Name,
			Path: hi.FileNameOrUrl,
		})
	}
	return items
}

// writeJSON serialises v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Error("Remote API: failed to write JSON response", "error", err)
	}
}
