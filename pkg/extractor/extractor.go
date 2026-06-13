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
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"wails-cast/pkg/folders"
)

const (
	WaitAfterManifestFound = 2 * time.Second
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

// Extract navigates to the page and extracts the HLS stream.
// It first looks for a Lua script in {configDir}/scripts/{domain}.lua.
// If found, uses headless browser with Lua automation.
// Otherwise, uses headful browser with generic request interception.
func Extract(pageURL string) (*ExtractResult, error) {
	scriptPath := findScript(pageURL)
	if scriptPath != "" {
		fmt.Printf("Found Lua script: %s\n", scriptPath)
		return extractWithScript(pageURL, scriptPath)
	}
	fmt.Println("No Lua script found, using headful extraction")
	return extractGeneric(pageURL)
}

func extractWithScript(pageURL string, scriptPath string) (*ExtractResult, error) {
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).Headless(true).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")
	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
	})
	page.MustEvalOnNewDocument(`
		HTMLMediaElement.prototype.canPlayType = function(type) {
			if (type && (type.includes('video') || type.includes('audio'))) return 'probably'; return '';
		};
		if (window.MediaSource) {
			MediaSource.isTypeSupported = function(type) {
				if (type && (type.includes('video') || type.includes('audio'))) return true; return false;
			};
		}
	`)

	L := lua.NewState()
	defer L.Close()

	registerFuncs(L, page)

	if err := L.DoFile(scriptPath); err != nil {
		return nil, fmt.Errorf("load script: %w", err)
	}

	var blockURLs []string
	if tbl := L.GetGlobal("block_urls"); tbl != lua.LNil {
		if t, ok := tbl.(*lua.LTable); ok {
			t.ForEach(func(_, value lua.LValue) {
				blockURLs = append(blockURLs, lua.LVAsString(value))
			})
		}
	}

	type interceptRule struct {
		pattern string
		handler lua.LValue
	}
	var intercepts []interceptRule
	if tbl := L.GetGlobal("intercepts"); tbl != lua.LNil {
		if t, ok := tbl.(*lua.LTable); ok {
			t.ForEach(func(_, value lua.LValue) {
				if rule, ok := value.(*lua.LTable); ok {
					p := rule.RawGetString("pattern")
					h := rule.RawGetString("handler")
					intercepts = append(intercepts, interceptRule{
						pattern: lua.LVAsString(p),
						handler: h,
					})
				}
			})
		}
	}

	var hlsURL string

	router := page.HijackRequests()
	router.MustAdd("*", func(ctx *rod.Hijack) {
		reqURL := ctx.Request.URL().String()

		for _, b := range blockURLs {
			if strings.Contains(reqURL, b) {
				ctx.Response.SetHeader("Content-Type", "application/javascript")
				ctx.Response.SetBody("// blocked")
				return
			}
		}

		for _, rule := range intercepts {
			if strings.Contains(reqURL, rule.pattern) {
				client := &http.Client{
					Transport:     &http.Transport{DisableKeepAlives: true},
					CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
				}
				if err := ctx.LoadResponse(client, true); err != nil {
					ctx.ContinueRequest(&proto.FetchContinueRequest{})
					return
				}
				body := ctx.Response.Body()

				L.Push(rule.handler)
				L.Push(lua.LString(reqURL))
				L.Push(lua.LString(body))
				if err := L.PCall(2, 1, nil); err == nil {
					result := L.Get(-1)
					L.Pop(1)
					if result != lua.LNil {
						hlsURL = lua.LVAsString(result)
						fmt.Printf("Lua intercept returned HLS URL: %s\n", hlsURL)
					}
				}
				return
			}
		}

		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})
	go router.Run()

	page.EnableDomain(proto.NetworkEnable{})
	cdpWait := page.EachEvent(func(e *proto.NetworkResponseReceived) bool {
		ct := e.Response.MIMEType
		if hlsURL == "" && (strings.Contains(ct, "application/vnd.apple.mpegurl") ||
			strings.Contains(ct, "application/x-mpegURL") ||
			strings.Contains(ct, "mpegurl")) {
			hlsURL = e.Response.URL
			fmt.Printf("CDP detected HLS: %s (CT: %s)\n", hlsURL, ct)
			return true
		}
		return false
	})
	go cdpWait()

	fmt.Printf("Navigating to %s...\n", pageURL)
	page.MustNavigate(pageURL)
	time.Sleep(2 * time.Second)
	fmt.Printf("Page loaded. Title: %s\n", page.MustInfo().Title)

	onReady := L.GetGlobal("on_ready")
	if onReady != lua.LNil {
		L.Push(onReady)
		if err := L.PCall(0, 0, nil); err != nil {
			fmt.Printf("Lua on_ready error: %v\n", err)
		}
	}

	fmt.Println("Waiting for HLS URL...")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if hlsURL != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if hlsURL == "" {
		return nil, fmt.Errorf("timeout waiting for HLS URL")
	}

	fmt.Printf("Loading HLS manifest: %s\n", hlsURL)
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", hlsURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15")
	// Use the HLS URL's own origin as referer (matches the domain the content is served from)
	if parsedHLS, err := url.Parse(hlsURL); err == nil {
		req.Header.Set("Referer", parsedHLS.Scheme+"://"+parsedHLS.Host+"/")
		req.Header.Set("Origin", parsedHLS.Scheme+"://"+parsedHLS.Host)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	defer resp.Body.Close()
	manifestBytes, _ := io.ReadAll(resp.Body)

	parsedURL, _ := url.Parse(hlsURL)
	result := &ExtractResult{
		URL:      parsedURL,
		Title:    page.MustInfo().Title,
		Manifest: string(manifestBytes),
		Cookies:  make(map[string]string),
		Headers:  make(map[string]string),
	}
	for _, c := range resp.Cookies() {
		result.Cookies[c.Name] = c.Value
	}
	result.Headers["User-Agent"] = req.UserAgent()
	result.Headers["Referer"] = req.Header.Get("Referer")
	result.Headers["Origin"] = req.Header.Get("Origin")

	fmt.Printf("Manifest loaded: %d bytes\n", len(manifestBytes))
	return result, nil
}

func extractGeneric(pageURL string) (*ExtractResult, error) {
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).Headless(false).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")

	var result = &ExtractResult{}
	var manifestFound bool
	subtitleURLs := make(map[string]bool)

	router := page.HijackRequests()
	router.MustAdd("*", func(ctx *rod.Hijack) {
		err := ctx.LoadResponse(http.DefaultClient, true)
		if err != nil {
			return
		}
		contentType := ctx.Response.Headers().Get("Content-Type")
		if !manifestFound {
			if strings.Contains(contentType, "application/vnd.apple.mpegurl") ||
				strings.Contains(contentType, "application/x-mpegURL") {
				reqURL := ctx.Request.URL()
				fmt.Printf("Found HLS stream: %s (Content-Type: %s)\n", reqURL, contentType)
				manifestContent := ctx.Response.Body()
				fmt.Printf("Manifest content length: %d bytes\n", len(manifestContent))
				cookies := make(map[string]string)
				cookieHeader := ctx.Request.Header("Cookie")
				if cookieHeader != "" {
					for cookie := range strings.SplitSeq(cookieHeader, "; ") {
						parts := strings.SplitN(cookie, "=", 2)
						if len(parts) == 2 {
							cookies[parts[0]] = parts[1]
						}
					}
				}
				headers := make(map[string]string)
				for _, h := range []string{"User-Agent", "Referer", "Origin"} {
					if v := ctx.Request.Header(h); v != "" {
						headers[h] = v
					}
				}
				result.Cookies = cookies
				result.Headers = headers
				result.URL = reqURL
				result.Manifest = manifestContent
				manifestFound = true
			}
		}
		if strings.Contains(strings.ToLower(contentType), "text/vtt") {
			reqURL := ctx.Request.URL().String()
			if subtitleURLs[reqURL] {
				return
			}
			subtitleURLs[reqURL] = true
			subtitleContent := ctx.Response.Body()
			label := ""
			if p, err := url.Parse(reqURL); err == nil {
				parts := strings.Split(p.Path, "/")
				label = strings.TrimSuffix(parts[len(parts)-1], ".vtt")
			}
			result.Subtitles = append(result.Subtitles, ExtractedSubtitleTrack{
				URL:     reqURL,
				Content: subtitleContent,
				Label:   label,
			})
		}
	})
	go router.Run()

	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
	})
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

	fmt.Printf("Navigating to %s...\n", pageURL)
	page.MustNavigate(pageURL)
	page.MustWaitLoad()
	fmt.Printf("Page loaded. Title: %s\n", page.MustInfo().Title)

	// Try common play button selectors
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

	done := make(chan struct{})
	go func() {
		for range time.NewTicker(100 * time.Millisecond).C {
			if manifestFound {
				select {
				case <-done:
					return
				default:
					close(done)
					return
				}
			}
		}
	}()

	fmt.Println("Waiting for video URL...")
	select {
	case <-done:
	case <-time.After(5 * time.Minute):
	}

	if !manifestFound {
		return nil, fmt.Errorf("timeout waiting for video URL")
	}

	result.Title = page.MustInfo().Title
	return result, nil
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
