package extractor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"wails-cast/pkg/folders"
)

const (
	WaitAfterManifestFound = 2 * time.Second

	browserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15"
)

type ExtractedSubtitleTrack struct {
	URL     string
	Content string `json:"-"`
	Charset string
	Label   string
}

type ExtractResult struct {
	URL       *url.URL
	Title     string
	Manifest  string `json:"-"`
	Cookies   map[string]string
	Headers   map[string]string
	Subtitles []ExtractedSubtitleTrack
}

type PlaylistExtractionResult struct {
	URL      *url.URL
	Cookies  map[string]string
	Headers  map[string]string
	Manifest string
}

type interceptRule struct {
	pattern string
	handler lua.LValue
}

// Extract navigates to the page and extracts the HLS stream.
//
// It first looks for a Lua script in {configDir}/scripts/ whose match_patterns
// match the URL. Whether or not a script is found, the same core extraction
// runs: a headless (scripted) or headful (generic) browser drives the page, a
// hijack router captures the manifest body and the *real* request headers the
// browser used (Referer/Origin/Cookie), and VTT subtitle tracks are collected.
//
// A matching Lua script only augments this with automation: block_urls,
// intercepts (pull the HLS URL out of a non-HLS response body), and on_ready
// (page interaction). Each of those defaults to the generic behavior when the
// script omits it.
func Extract(pageURL string) (*ExtractResult, error) {
	scriptPath := findScript(pageURL)
	if scriptPath != "" {
		fmt.Printf("Found Lua script: %s\n", scriptPath)
	} else {
		fmt.Println("No Lua script found, using generic extraction")
	}
	return extract(pageURL, scriptPath)
}

func extract(pageURL string, scriptPath string) (*ExtractResult, error) {
	scripted := scriptPath != ""

	path, _ := launcher.LookPath()
	// Scripted extractions automate the play interaction, so they run headless.
	// Without a script a human may need to click play, so run headful.
	u := launcher.New().Bin(path).Headless(scripted).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")
	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: browserUserAgent})
	page.MustEvalOnNewDocument(`
		HTMLMediaElement.prototype.canPlayType = function(type) {
			if (type && (type.includes('video') || type.includes('audio'))) return 'probably';
			return '';
		};
		if (window.MediaSource) {
			MediaSource.isTypeSupported = function(type) {
				if (type && (type.includes('video') || type.includes('audio'))) return true;
				return false;
			};
		}
	`)

	// Lua automation (only when a script matched). Everything below defaults to
	// generic behavior when the corresponding global is absent.
	var (
		L          *lua.LState
		blockURLs  []string
		intercepts []interceptRule
		onReady    lua.LValue = lua.LNil
	)
	if scripted {
		L = lua.NewState()
		defer L.Close()
		registerFuncs(L, page)
		if err := L.DoFile(scriptPath); err != nil {
			return nil, fmt.Errorf("load script: %w", err)
		}
		if tbl, ok := L.GetGlobal("block_urls").(*lua.LTable); ok {
			tbl.ForEach(func(_, v lua.LValue) { blockURLs = append(blockURLs, lua.LVAsString(v)) })
		}
		if tbl, ok := L.GetGlobal("intercepts").(*lua.LTable); ok {
			tbl.ForEach(func(_, v lua.LValue) {
				if rule, ok := v.(*lua.LTable); ok {
					intercepts = append(intercepts, interceptRule{
						pattern: lua.LVAsString(rule.RawGetString("pattern")),
						handler: rule.RawGetString("handler"),
					})
				}
			})
		}
		onReady = L.GetGlobal("on_ready")
	}

	// Shared capture state, guarded by mu (hijack handlers run concurrently).
	var (
		mu            sync.Mutex
		manifestFound bool
		hlsURL        string
		manifestBody  string // captured inline; empty means "fetch hlsURL below"
		capReferer    string
		capOrigin     string
		capCookie     string
		subtitleSeen  = map[string]bool{}
		subtitles     []ExtractedSubtitleTrack
	)

	client := &http.Client{Timeout: 20 * time.Second}

	router := page.HijackRequests()
	router.MustAdd("*", func(ctx *rod.Hijack) {
		reqURL := ctx.Request.URL().String()

		// block_urls (Lua): replace matching requests with an empty JS file.
		for _, b := range blockURLs {
			if strings.Contains(reqURL, b) {
				ctx.Response.SetHeader("Content-Type", "application/javascript")
				ctx.Response.SetBody("// blocked")
				return
			}
		}

		// Fetch the response server-side so we can inspect its body/headers.
		if err := ctx.LoadResponse(client, true); err != nil {
			ctx.ContinueRequest(&proto.FetchContinueRequest{})
			return
		}
		contentType := ctx.Response.Headers().Get("Content-Type")

		mu.Lock()
		defer mu.Unlock()

		// Lua intercepts: a handler may pull the HLS URL out of a non-HLS body
		// (e.g. a JSON API, or an embed page's HTML). It returns the URL, which
		// we fetch further below. We do NOT capture this request's Referer: the
		// intercepted request is not the manifest request, so its referer isn't
		// the manifest's. The fetch below defaults to the m3u8's own origin.
		if !manifestFound {
			for _, rule := range intercepts {
				if !strings.Contains(reqURL, rule.pattern) {
					continue
				}
				L.Push(rule.handler)
				L.Push(lua.LString(reqURL))
				L.Push(lua.LString(ctx.Response.Body()))
				if err := L.PCall(2, 1, nil); err == nil {
					res := L.Get(-1)
					L.Pop(1)
					if res != lua.LNil {
						hlsURL = lua.LVAsString(res)
						manifestFound = true
						fmt.Printf("Lua intercept returned HLS URL: %s\n", hlsURL)
					}
				}
				break
			}
		}

		// Default detection: capture the manifest body and the real headers the
		// browser used. This is the correct Referer — the m3u8's own origin is
		// often wrong (embed on domain A, CDN on domain B gated by Referer: A).
		if !manifestFound && (strings.Contains(contentType, "application/vnd.apple.mpegurl") ||
			strings.Contains(contentType, "application/x-mpegURL") ||
			strings.Contains(contentType, "mpegurl")) {
			hlsURL = reqURL
			manifestBody = ctx.Response.Body()
			capReferer = ctx.Request.Header("Referer")
			capOrigin = ctx.Request.Header("Origin")
			capCookie = ctx.Request.Header("Cookie")
			manifestFound = true
			fmt.Printf("Found HLS stream: %s (Content-Type: %s, Referer: %s)\n", hlsURL, contentType, capReferer)
		}

		// Subtitles (VTT), collected regardless of scripted/generic.
		if strings.Contains(strings.ToLower(contentType), "text/vtt") && !subtitleSeen[reqURL] {
			subtitleSeen[reqURL] = true
			label := ""
			if p, err := url.Parse(reqURL); err == nil {
				parts := strings.Split(p.Path, "/")
				label = strings.TrimSuffix(parts[len(parts)-1], ".vtt")
			}
			subtitles = append(subtitles, ExtractedSubtitleTrack{
				URL:     reqURL,
				Content: ctx.Response.Body(),
				Label:   label,
			})
		}
	})
	go router.Run()

	fmt.Printf("Navigating to %s...\n", pageURL)
	page.MustNavigate(pageURL)
	page.MustWaitLoad()
	fmt.Printf("Page loaded. Title: %s\n", page.MustInfo().Title)

	// Interaction: run the Lua on_ready if present, otherwise the generic
	// best-effort click of common play-button selectors.
	if onReady != lua.LNil {
		L.Push(onReady)
		if err := L.PCall(0, 0, nil); err != nil {
			fmt.Printf("Lua on_ready error: %v\n", err)
		}
	} else {
		for _, sel := range []string{"#play-video", ".play-button", ".video-play-button"} {
			el, err := page.Timeout(3 * time.Second).Element(sel)
			if err == nil {
				fmt.Printf("Clicking play button: %s\n", sel)
				box := el.MustShape().Box()
				page.Mouse.MustMoveTo(box.X+box.Width/2, box.Y+box.Height/2)
				page.Mouse.MustDown("left")
				page.Mouse.MustUp("left")
				time.Sleep(2 * time.Second)
				break
			}
		}
	}

	// Wait for the manifest. Headful/generic waits longer for a manual click.
	timeout := 30 * time.Second
	if !scripted {
		timeout = 5 * time.Minute
	}
	fmt.Println("Waiting for HLS URL...")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		found := manifestFound
		mu.Unlock()
		if found {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Snapshot the captured values.
	mu.Lock()
	found := manifestFound
	sHlsURL, sBody, sRef, sOrg, sCookie := hlsURL, manifestBody, capReferer, capOrigin, capCookie
	sSubs := append([]ExtractedSubtitleTrack(nil), subtitles...)
	mu.Unlock()

	if !found {
		return nil, fmt.Errorf("timeout waiting for HLS URL")
	}

	cookies := map[string]string{}

	// An intercept returned only a URL — fetch it now, carrying the real
	// Referer/Origin/Cookie from the request that revealed it (falling back to
	// the m3u8's own origin only if we captured nothing).
	if sBody == "" {
		fmt.Printf("Loading HLS manifest: %s\n", sHlsURL)
		req, _ := http.NewRequest("GET", sHlsURL, nil)
		req.Header.Set("User-Agent", browserUserAgent)
		if sRef != "" {
			req.Header.Set("Referer", sRef)
			if sOrg != "" {
				req.Header.Set("Origin", sOrg)
			}
		} else if parsed, err := url.Parse(sHlsURL); err == nil {
			req.Header.Set("Referer", parsed.Scheme+"://"+parsed.Host+"/")
			req.Header.Set("Origin", parsed.Scheme+"://"+parsed.Host)
		}
		if sCookie != "" {
			req.Header.Set("Cookie", sCookie)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("load manifest: %w", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		sBody = string(b)
		for _, c := range resp.Cookies() {
			cookies[c.Name] = c.Value
		}
		sRef = req.Header.Get("Referer")
		sOrg = req.Header.Get("Origin")
	}

	// Cookies from the captured request Cookie header.
	if sCookie != "" {
		for _, cookie := range strings.Split(sCookie, "; ") {
			parts := strings.SplitN(cookie, "=", 2)
			if len(parts) == 2 {
				cookies[parts[0]] = parts[1]
			}
		}
	}

	headers := map[string]string{"User-Agent": browserUserAgent}
	if sRef != "" {
		headers["Referer"] = sRef
	}
	if sOrg != "" {
		headers["Origin"] = sOrg
	}

	parsedURL, _ := url.Parse(sHlsURL)
	fmt.Printf("Manifest loaded: %d bytes\n", len(sBody))
	return &ExtractResult{
		URL:       parsedURL,
		Title:     page.MustInfo().Title,
		Manifest:  sBody,
		Cookies:   cookies,
		Headers:   headers,
		Subtitles: sSubs,
	}, nil
}

// findScript scans all Lua scripts for match_patterns matching the URL.
func findScript(pageURL string) string {
	scriptsDir := filepath.Join(folders.GetConfig(), "scripts")
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
			continue
		}
		scriptPath := filepath.Join(scriptsDir, entry.Name())
		if matchesURLPattern(scriptPath, pageURL) {
			return scriptPath
		}
	}
	return ""
}

// matchesURLPattern checks if a script's match_patterns table contains
// a wildcard pattern matching the given URL.
// Patterns are matched against the full URL, then against host+path.
func matchesURLPattern(scriptPath, pageURL string) bool {
	L := lua.NewState()
	defer L.Close()
	if err := L.DoFile(scriptPath); err != nil {
		return false
	}
	tbl := L.GetGlobal("match_patterns")
	if tbl == lua.LNil {
		return false
	}
	t, ok := tbl.(*lua.LTable)
	if !ok {
		return false
	}
	// Also try matching against just scheme+host+path (no query/fragment)
	parsed, _ := url.Parse(pageURL)
	hostPath := parsed.Host + parsed.Path
	hostPath = strings.TrimPrefix(hostPath, "www.")

	var matched bool
	t.ForEach(func(_, value lua.LValue) {
		pattern := lua.LVAsString(value)
		if pattern == "" {
			return
		}
		if wildcardMatch(pattern, pageURL) || wildcardMatch(pattern, hostPath) {
			matched = true
		}
	})
	return matched
}

// wildcardMatch checks if text matches a pattern containing * wildcards.
func wildcardMatch(pattern, text string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == text
	}
	if !strings.HasPrefix(text, parts[0]) {
		return false
	}
	text = text[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(text, parts[i])
		if idx < 0 {
			return false
		}
		text = text[idx+len(parts[i]):]
	}
	return strings.HasSuffix(text, parts[len(parts)-1])
}

func registerFuncs(L *lua.LState, page *rod.Page) {
	L.SetGlobal("wait_element", L.NewFunction(func(L *lua.LState) int {
		selector := L.CheckString(1)
		timeout := L.OptInt(2, 10)
		_, err := page.Timeout(time.Duration(timeout) * time.Second).Element(selector)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetGlobal("click", L.NewFunction(func(L *lua.LState) int {
		selector := L.CheckString(1)
		el, err := page.Element(selector)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		box := el.MustShape().Box()
		page.Mouse.MustMoveTo(box.X+box.Width/2, box.Y+box.Height/2)
		page.Mouse.MustDown("left")
		page.Mouse.MustUp("left")
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetGlobal("sleep", L.NewFunction(func(L *lua.LState) int {
		secs := L.CheckNumber(1)
		time.Sleep(time.Duration(secs*1000) * time.Millisecond)
		return 0
	}))

	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		fmt.Println("[LUA]", L.CheckString(1))
		return 0
	}))

	L.SetGlobal("js", L.NewFunction(func(L *lua.LState) int {
		code := L.CheckString(1)
		result, err := page.Eval(code)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		_ = result
		return 1
	}))

	L.SetGlobal("scroll_into_view", L.NewFunction(func(L *lua.LState) int {
		selector := L.CheckString(1)
		_, err := page.Eval(fmt.Sprintf(`() => {
			var el = document.querySelector('%s');
			if (el) el.scrollIntoView({behavior: 'instant', block: 'center'});
		}`, selector))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		time.Sleep(500 * time.Millisecond)
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetGlobal("json_decode", L.NewFunction(func(L *lua.LState) int {
		s := L.CheckString(1)
		val, err := decodeJSON(L, s)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(val)
		return 1
	}))
}

func decodeJSON(L *lua.LState, s string) (lua.LValue, error) {
	var val interface{}
	if err := json.Unmarshal([]byte(s), &val); err != nil {
		return lua.LNil, err
	}
	return toLuaValue(L, val), nil
}

func toLuaValue(L *lua.LState, v interface{}) lua.LValue {
	switch vv := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(vv)
	case float64:
		return lua.LNumber(vv)
	case string:
		return lua.LString(vv)
	case []interface{}:
		tbl := L.NewTable()
		for i, item := range vv {
			tbl.RawSetInt(i+1, toLuaValue(L, item))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, val := range vv {
			tbl.RawSetString(k, toLuaValue(L, val))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", vv))
	}
}
