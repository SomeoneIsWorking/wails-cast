package main

// HTTPServer exposes a small REST/JSON API so remote clients (e.g. an Android
// companion app) can browse the media library, trigger playback of a library
// item, and send an arbitrary URL to be played on this desktop.
//
// Endpoints
//   GET  /ping          – health / discovery
//   GET  /library       – list library items (delegates to LibraryLister)
//   GET  /devices       – list cast targets (incl. "local") for the picker
//   GET  /state         – current playback state snapshot
//   GET  /track-info    – video/audio/subtitle tracks for a media item
//   POST /play          – play a library item by id (+ track/subtitle/quality)
//   POST /play-url      – play an arbitrary URL (+ track/subtitle/quality)
//   POST /control       – transport: pause/resume/stop/seek/volume/mute
//   POST /translate     – start a subtitle translation for a media item
//   GET  /translate-status – poll translation progress
//   GET  /translation-info – whether a translation already exists for an item
//   GET/POST /settings  – read/update subtitle size, quality, language
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
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"

	"wails-cast/pkg/events"
	"wails-cast/pkg/options"
	"wails-cast/pkg/castapi"
)

// mdnsService is the mDNS/Bonjour service type the desktop advertises so the
// companion app can discover it automatically on the LAN.
const mdnsService = "_wailscast._tcp"

// ----------------------------------------------------------------------------
// Library abstraction (consumed by /library)
// ----------------------------------------------------------------------------

// LibraryItem is the shape returned to remote clients for a single media item.
type LibraryItem = castapi.LibraryItem

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

// playOptions carries the optional track/subtitle/quality selection shared by
// POST /play and POST /play-url. All fields are optional; omitted fields fall
// back to sensible defaults (track 0, no subtitle, default quality setting).
type playOptions struct {
	VideoTrack   int     `json:"videoTrack"`
	AudioTrack   int     `json:"audioTrack"`
	SubtitlePath string  `json:"subtitlePath"` // "" or "none" = no subtitle
	Quality      *string `json:"quality"`      // nil = default setting; "" = Original
}

// playRequest is the body for POST /play.
type playRequest struct {
	ID       string `json:"id"`       // library item id (== path)
	DeviceIP string `json:"deviceIp"` // target device; "local" for desktop-only
	playOptions
}

// playURLRequest is the body for POST /play-url.
type playURLRequest struct {
	URL      string `json:"url"`
	DeviceIP string `json:"deviceIp"` // target device; "local" for desktop-only
	playOptions
}

// playResponse is returned after a successful play command.
type playResponse struct {
	OK    bool          `json:"ok"`
	State PlaybackState `json:"state"`
}

// errorResponse is the standard error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// deviceItem is a cast target returned by GET /devices. Host is the value the
// client passes back as deviceIp on /play and /play-url ("local" = desktop).
type deviceItem struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
	UUID string `json:"uuid"`
}

// devicesResponse wraps the device list.
type devicesResponse struct {
	Items []deviceItem `json:"items"`
}

// controlRequest is the body for POST /control. Action selects the transport
// command; Value carries the numeric argument for seek (seconds) and volume
// (0.0–1.0) and is ignored otherwise.
type controlRequest struct {
	Action string  `json:"action"`
	Value  float64 `json:"value"`
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
	mdns     *zeroconf.Server // pure-Go advertiser (non-darwin)
	mdnsCmd  *exec.Cmd        // system `dns-sd -R` advertiser (darwin)
	mu       sync.Mutex
	running  bool

	unsubscribe func()          // event-bus subscription teardown
	transMu     sync.Mutex      // guards transStatus
	transStatus translateStatus // latest translation progress, for /translate-status

	qbtMu  sync.Mutex // guards qbt + qbtKey
	qbt    *qbtClient // lazily built qBittorrent client
	qbtKey string     // url|user|pass the cached client was built for

	seasonMu     sync.Mutex              // guards seasonStatus
	seasonStatus SeasonTranslateProgress // latest season-translation progress
}

// translateStatus mirrors the translation lifecycle (driven by the event bus)
// so remote clients can poll progress via GET /translate-status.
type translateStatus struct {
	InProgress bool     `json:"inProgress"`
	Language   string   `json:"language"`
	Files      []string `json:"files"`
	Error      string   `json:"error"`
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
	mux.HandleFunc("/devices", h.handleDevices)
	mux.HandleFunc("/state", h.handleState)
	mux.HandleFunc("/track-info", h.handleTrackInfo)
	mux.HandleFunc("/play", h.handlePlay)
	mux.HandleFunc("/play-url", h.handlePlayURL)
	mux.HandleFunc("/control", h.handleControl)
	mux.HandleFunc("/translate", h.handleTranslate)
	mux.HandleFunc("/translate-status", h.handleTranslateStatus)
	mux.HandleFunc("/translation-info", h.handleTranslationInfo)
	mux.HandleFunc("/settings", h.handleSettings)
	mux.HandleFunc("/torrent/add", h.handleTorrentAdd)
	mux.HandleFunc("/torrents", h.handleTorrents)
	mux.HandleFunc("/library/tree", h.handleLibraryTree)
	mux.HandleFunc("/library/identify", h.handleLibraryIdentify)
	mux.HandleFunc("/library/organize/preview", h.handleOrganizePreview)
	mux.HandleFunc("/library/organize/execute", h.handleOrganizeExecute)
	mux.HandleFunc("/library/translate-season", h.handleTranslateSeason)
	mux.HandleFunc("/library/translate-season/status", h.handleSeasonStatus)
	mux.HandleFunc("/library/translate-season/cancel", h.handleSeasonCancel)
	mux.HandleFunc("/subtitle", h.handleSubtitle)

	h.listener = ln
	h.srv = &http.Server{
		Handler:      h.withMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	h.running = true

	// Track translation lifecycle so /translate-status can report progress.
	h.unsubscribe = events.Subscribe(func(topic string, payload any) {
		switch topic {
		case "translation:complete":
			files, _ := payload.([]string)
			h.transMu.Lock()
			h.transStatus.InProgress = false
			h.transStatus.Files = files
			h.transStatus.Error = ""
			h.transMu.Unlock()
		case "translation:error":
			msg, _ := payload.(string)
			h.transMu.Lock()
			h.transStatus.InProgress = false
			h.transStatus.Error = msg
			h.transMu.Unlock()
		case "translation:cancelled":
			h.transMu.Lock()
			h.transStatus.InProgress = false
			h.transStatus.Error = "cancelled"
			h.transMu.Unlock()
		case "library:translate:progress":
			if p, ok := payload.(SeasonTranslateProgress); ok {
				h.seasonMu.Lock()
				h.seasonStatus = p
				h.seasonMu.Unlock()
			}
		}
	})

	go func() {
		logger.Info("Remote API listening", "addr", addr)
		if err := h.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("Remote API server error", "error", err)
		}
	}()

	// Advertise over mDNS so the companion app can auto-discover this host.
	instance := strings.TrimSuffix(hostnameOrDefault(), ".local")
	h.startMDNS(instance, port)

	return nil
}

// hostnameOrDefault returns the OS hostname, falling back to "wails-cast".
func hostnameOrDefault() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "wails-cast"
}

// startMDNS advertises the remote API over mDNS. On macOS it registers through
// the system mDNS responder via the `dns-sd` CLI: a pure-Go responder bound to
// UDP 5353 is not discoverable while macOS's own mDNSResponder owns the port.
// On other platforms grandcat/zeroconf is used.
func (h *HTTPServer) startMDNS(instance string, port int) {
	if runtime.GOOS == "darwin" {
		if path, err := exec.LookPath("dns-sd"); err == nil {
			cmd := exec.Command(path, "-R", instance, mdnsService, "local", strconv.Itoa(port), "app=wails-cast")
			if err := cmd.Start(); err != nil {
				logger.Warn("Remote API: dns-sd register failed", "error", err)
			} else {
				h.mdnsCmd = cmd
				logger.Info("Remote API mDNS advertised (dns-sd)", "instance", instance, "service", mdnsService, "port", port)
			}
			return
		}
	}
	if mdns, err := zeroconf.Register(instance, mdnsService, "local.", port, []string{"app=wails-cast"}, nil); err != nil {
		logger.Warn("Remote API: mDNS advertisement failed", "error", err)
	} else {
		h.mdns = mdns
		logger.Info("Remote API mDNS advertised (zeroconf)", "instance", instance, "service", mdnsService, "port", port)
	}
}

// stopMDNS tears down whichever mDNS advertiser is active.
func (h *HTTPServer) stopMDNS() {
	if h.mdnsCmd != nil && h.mdnsCmd.Process != nil {
		_ = h.mdnsCmd.Process.Kill()
		_ = h.mdnsCmd.Wait()
		h.mdnsCmd = nil
	}
	if h.mdns != nil {
		h.mdns.Shutdown()
		h.mdns = nil
	}
}

// Stop shuts the server down gracefully.
func (h *HTTPServer) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}
	h.running = false

	if h.unsubscribe != nil {
		h.unsubscribe()
		h.unsubscribe = nil
	}
	h.stopMDNS()
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

	state, err := h.app.CastToDevice(deviceIP, req.ID, h.castOptions(req.playOptions))
	if err != nil {
		logger.Error("Remote API: play failed", "id", req.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, playResponse{OK: true, State: *state})
}

// castOptions converts the optional remote playOptions into options.CastOptions,
// applying defaults for any field the client left unset.
func (h *HTTPServer) castOptions(p playOptions) *options.CastOptions {
	subtitle := p.SubtitlePath
	if subtitle == "" {
		subtitle = "none"
	}
	bitrate := h.app.settingsStore.Get().DefaultQuality
	if p.Quality != nil {
		bitrate = *p.Quality
	}
	return &options.CastOptions{
		SubtitlePath: subtitle,
		VideoTrack:   p.VideoTrack,
		AudioTrack:   p.AudioTrack,
		Bitrate:      bitrate,
	}
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

	state, err := h.app.CastToDevice(deviceIP, req.URL, h.castOptions(req.playOptions))
	if err != nil {
		logger.Error("Remote API: play-url failed", "url", req.URL, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, playResponse{OK: true, State: *state})
}

// handleDevices runs a synchronous discovery and returns the available cast
// targets, always including the desktop itself ("local") as the first entry.
func (h *HTTPServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	items := []deviceItem{{Name: "This Computer", Host: "local", Port: 0, UUID: "local"}}
	for _, d := range h.app.discovery.DiscoverSync(3 * time.Second) {
		items = append(items, deviceItem{Name: d.Name, Host: d.Host, Port: d.Port, UUID: d.UUID})
	}
	writeJSON(w, http.StatusOK, devicesResponse{Items: items})
}

// handleState returns the current playback state snapshot.
func (h *HTTPServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	h.app.mu.RLock()
	state := h.app.playbackState
	h.app.mu.RUnlock()
	writeJSON(w, http.StatusOK, state)
}

// handleControl applies a transport command (pause/resume/stop/seek/volume/mute)
// to the active playback session and returns the resulting state.
func (h *HTTPServer) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req controlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	var err error
	switch strings.ToLower(req.Action) {
	case "pause":
		err = h.app.Pause()
	case "resume", "unpause", "play":
		err = h.app.Unpause()
	case "stop":
		err = h.app.StopPlayback()
	case "seek":
		err = h.app.SeekTo(req.Value)
	case "volume":
		err = h.app.SetVolume(float32(req.Value))
	case "mute":
		err = h.app.SetMuted(true)
	case "unmute":
		err = h.app.SetMuted(false)
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unknown action: " + req.Action})
		return
	}

	if err != nil {
		logger.Error("Remote API: control failed", "action", req.Action, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	h.app.mu.RLock()
	state := h.app.playbackState
	h.app.mu.RUnlock()
	writeJSON(w, http.StatusOK, playResponse{OK: true, State: state})
}

// handleTrackInfo returns the video/audio/subtitle tracks for a media item so
// the client can offer track and subtitle pickers before playing.
// GET /track-info?id=<path-or-url>
func (h *HTTPServer) handleTrackInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id query parameter is required"})
		return
	}
	info, err := h.app.GetTrackDisplayInfo(id)
	if err != nil {
		logger.Error("Remote API: track-info failed", "id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// translateRequest is the body for POST /translate.
type translateRequest struct {
	ID       string `json:"id"`       // media file path
	Language string `json:"language"` // target language; "" = default setting
}

// handleTranslate starts an asynchronous subtitle translation for a media item.
// Progress is reported via GET /translate-status.
func (h *HTTPServer) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req translateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id is required"})
		return
	}
	lang := req.Language
	if lang == "" {
		lang = h.app.settingsStore.Get().DefaultTranslationLanguage
	}

	// Mark in-progress before kicking off so an immediate poll reflects it.
	h.transMu.Lock()
	h.transStatus = translateStatus{InProgress: true, Language: lang}
	h.transMu.Unlock()

	if err := h.app.TranslateExportedSubtitles(req.ID, lang); err != nil {
		h.transMu.Lock()
		h.transStatus = translateStatus{InProgress: false, Language: lang, Error: err.Error()}
		h.transMu.Unlock()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "language": lang})
}

// handleTranslationInfo reports whether a translated subtitle already exists for
// a media item in the given language (defaulting to the configured language).
// GET /translation-info?id=<path>&language=<lang>
func (h *HTTPServer) handleTranslationInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id query parameter is required"})
		return
	}
	lang := r.URL.Query().Get("language")
	if lang == "" {
		lang = h.app.settingsStore.Get().DefaultTranslationLanguage
	}
	translated := hasTranslation(id, lang)
	path := ""
	if translated {
		path = translatedSubtitlePath(id, lang)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"language":     lang,
		"translated":   translated,
		"subtitlePath": path,
	})
}

// handleTranslateStatus reports the latest translation progress.
func (h *HTTPServer) handleTranslateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	h.transMu.Lock()
	status := h.transStatus
	h.transMu.Unlock()
	writeJSON(w, http.StatusOK, status)
}

// remoteSettings is the subset of desktop settings the remote can read/write.
type remoteSettings struct {
	SubtitleFontSize           int    `json:"subtitleFontSize"`
	SubtitleBurnIn             bool   `json:"subtitleBurnIn"`
	IgnoreClosedCaptions       bool   `json:"ignoreClosedCaptions"`
	DefaultQuality             string `json:"defaultQuality"`
	DefaultTranslationLanguage string `json:"defaultTranslationLanguage"`
}

// qualityOption mirrors the desktop's quality presets for the remote picker.
type qualityOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var remoteQualityOptions = []qualityOption{
	{Value: "", Label: "Original (Best Quality)"},
	{Value: "8M", Label: "Very High (8M)"},
	{Value: "5M", Label: "High (5M)"},
	{Value: "3M", Label: "Medium (3M)"},
	{Value: "2M", Label: "Low (2M)"},
}

// handleSettings exposes the remote-relevant settings subset.
// GET returns the current values plus the quality preset list; POST updates them.
func (h *HTTPServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s := h.app.settingsStore.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"settings": remoteSettings{
				SubtitleFontSize:           s.SubtitleFontSize,
				SubtitleBurnIn:             s.SubtitleBurnIn,
				IgnoreClosedCaptions:       s.IgnoreClosedCaptions,
				DefaultQuality:             s.DefaultQuality,
				DefaultTranslationLanguage: s.DefaultTranslationLanguage,
			},
			"qualityOptions": remoteQualityOptions,
		})
	case http.MethodPost:
		var in remoteSettings
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
			return
		}
		// Merge onto the full settings so we don't clobber desktop-only fields.
		full := *h.app.settingsStore.Get()
		full.SubtitleFontSize = in.SubtitleFontSize
		full.SubtitleBurnIn = in.SubtitleBurnIn
		full.IgnoreClosedCaptions = in.IgnoreClosedCaptions
		full.DefaultQuality = in.DefaultQuality
		full.DefaultTranslationLanguage = in.DefaultTranslationLanguage
		if err := h.app.UpdateSettings(full); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
	}
}

// ----------------------------------------------------------------------------
// Torrents (qBittorrent integration)
// ----------------------------------------------------------------------------

// qbtClientForSettings returns a qBittorrent client built from the current
// settings, rebuilding it when the URL/credentials change.
func (h *HTTPServer) qbtClientForSettings() (*qbtClient, error) {
	s := h.app.settingsStore.Get()
	key := s.QbtURL + "|" + s.QbtUser + "|" + s.QbtPass

	h.qbtMu.Lock()
	defer h.qbtMu.Unlock()
	if h.qbt != nil && h.qbtKey == key {
		return h.qbt, nil
	}
	c, err := newQbtClient(s.QbtURL, s.QbtUser, s.QbtPass)
	if err != nil {
		return nil, err
	}
	h.qbt = c
	h.qbtKey = key
	return c, nil
}

// torrentAddRequest is the body for POST /torrent/add.
type torrentAddRequest struct {
	Magnet string `json:"magnet"`
}

// handleTorrentAdd sends a magnet link to qBittorrent. The download is saved
// into QbtSavePath, falling back to the configured LibraryRoot so finished
// downloads land in the library.
func (h *HTTPServer) handleTorrentAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req torrentAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Magnet) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "magnet is required"})
		return
	}

	client, err := h.qbtClientForSettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	s := h.app.settingsStore.Get()
	savePath := s.QbtSavePath
	if savePath == "" {
		savePath = s.LibraryRoot
	}

	if err := client.AddMagnet(req.Magnet, savePath); err != nil {
		logger.Error("Remote API: torrent add failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "savePath": savePath})
}

// handleTorrents lists the current qBittorrent torrents with progress.
func (h *HTTPServer) handleTorrents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	client, err := h.qbtClientForSettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	torrents, err := client.Torrents()
	if err != nil {
		logger.Error("Remote API: list torrents failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": torrents})
}

// ----------------------------------------------------------------------------
// Library management (remote mode: scan tree / identify / organize / season-translate)
// ----------------------------------------------------------------------------

// handleLibraryTree returns the full Show/Season/Episode tree for the configured
// library root so remote controllers can render the same tree as the local app.
func (h *HTTPServer) handleLibraryTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	s := h.app.settingsStore.Get()
	if s.LibraryRoot == "" {
		writeJSON(w, http.StatusOK, &LibraryScanResult{Shows: []LibraryShow{}})
		return
	}
	result, err := scanLibraryRoot(s.LibraryRoot, s.DefaultTranslationLanguage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleLibraryIdentify enriches a posted scan result with TMDB metadata.
func (h *HTTPServer) handleLibraryIdentify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in LibraryScanResult
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	out, err := h.app.IdentifyLibrary(&in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleOrganizePreview builds the (non-destructive) organize move plan.
func (h *HTTPServer) handleOrganizePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in LibraryScanResult
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	plan, err := h.app.PreviewOrganize(&in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if plan == nil {
		plan = []OrganizeMove{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

// handleOrganizeExecute runs a previously-previewed organize plan.
func (h *HTTPServer) handleOrganizeExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in struct {
		Plan []OrganizeMove `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := h.app.OrganizeLibrary(in.Plan); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// seasonTranslateRequest is the body for POST /library/translate-season.
type seasonTranslateRequest struct {
	ShowName     string   `json:"showName"`
	SeasonName   string   `json:"seasonName"`
	EpisodePaths []string `json:"episodePaths"`
	Language     string   `json:"language"`
}

// handleTranslateSeason starts a season-batch translation; progress is polled via
// GET /library/translate-season/status.
func (h *HTTPServer) handleTranslateSeason(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req seasonTranslateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	lang := req.Language
	if lang == "" {
		lang = h.app.settingsStore.Get().DefaultTranslationLanguage
	}
	// Reset status so an immediate poll reflects the new run.
	h.seasonMu.Lock()
	h.seasonStatus = SeasonTranslateProgress{ShowName: req.ShowName, SeasonName: req.SeasonName, TargetLanguage: lang, TotalEpisodes: len(req.EpisodePaths), Status: "running"}
	h.seasonMu.Unlock()

	if err := h.app.TranslateSeason(req.ShowName, req.SeasonName, req.EpisodePaths, lang); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "language": lang})
}

// handleSeasonStatus reports the latest season-translation progress.
func (h *HTTPServer) handleSeasonStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	h.seasonMu.Lock()
	status := h.seasonStatus
	h.seasonMu.Unlock()
	writeJSON(w, http.StatusOK, status)
}

// handleSeasonCancel cancels an in-progress season-batch translation.
func (h *HTTPServer) handleSeasonCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	h.app.CancelSeasonTranslation()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleSubtitle applies live subtitle settings (size/sync/style) to the active
// playback on this instance — used by a remote controller's subtitle controls.
func (h *HTTPServer) handleSubtitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var opts options.SubtitleCastOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := h.app.UpdateSubtitleSettings(opts); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
