# AI Agent Workflow: Creating a Lua Script

This document describes the step-by-step process for an AI agent to probe
a website and create a Lua extraction script for it.

## Step 1: Probe the Page

Create and run a Go probe script to understand the page structure:

```go
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func main() {
	pageURL := "https://example.com/video-page"

	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).Headless(false).NoSandbox(true).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("")

	// Block known anti-bot scripts
	router := page.HijackRequests()
	router.MustAdd("*", func(ctx *rod.Hijack) {
		url := ctx.Request.URL().String()
		if strings.Contains(url, "disable-devtool") {
			ctx.Response.SetHeader("Content-Type", "application/javascript")
			ctx.Response.SetBody("// blocked")
			return
		}
		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})
	go router.Run()

	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
	})

	page.MustNavigate(pageURL)
	page.MustWaitLoad()
	fmt.Printf("Title: %s\n", page.MustInfo().Title)
	time.Sleep(3 * time.Second)

	// Collect page structure
	iframes, _ := page.Elements("iframe")
	fmt.Printf("Iframes: %d\n", len(iframes))
	for i, f := range iframes {
		src, _ := f.Attribute("src")
		id, _ := f.Attribute("id")
		fmt.Printf("  [%d] src=%s id=%s\n", i, src, id)
	}

	videos, _ := page.Elements("video")
	fmt.Printf("Videos: %d\n", len(videos))
	for i, v := range videos {
		src, _ := v.Attribute("src")
		fmt.Printf("  [%d] src=%s\n", i, src)
	}

	// Play buttons
	sel := page.Eval(`() => Array.from(document.querySelectorAll('[id*="play" i], [class*="play" i]')).map(e => e.tagName + "#" + e.id + "." + e.className)`)
	fmt.Printf("Play elements: %s\n", sel.Value.Str())

	// Check for JSON APIs in page source
	fmt.Printf("\nHTML preview:\n%s\n", page.MustHTML()[:2000])
}
```

Save this as `/tmp/probe_{site}.go` and run with `go run /tmp/probe_{site}.go`.

## Step 2: Identify the Video Source

From the probe output, determine how the video loads:

**Pattern A: Direct HLS manifest**
- If `<video>` element has an `src` ending in `.m3u8`, or if network
  responses show `Content-Type: application/vnd.apple.mpegurl`
- **Script approach**: CDP listener catches it automatically after click.
  Just need `on_ready()` to click play.

**Pattern B: JSON API**
- If a network request (e.g., `source2.php`, `api.php`) returns JSON
  containing `"file": "https://...m3u8"`
- **Script approach**: Use `intercepts` to capture the JSON response,
  parse it, and return the HLS URL.

**Pattern C: Iframe embed**
- If the video is in an iframe (e.g., `rapidvid.net`, `pichive.online`)
- **Script approach**: Click the play button on the parent page, then
  the CDP listener catches the HLS from the iframe.

## Step 3: Verify the Click Works

Write a focused probe to test the click:

```go
// After navigation, try clicking
page.Eval(`() => document.querySelector('#play-button').scrollIntoView({block:'center'})`)
time.Sleep(1 * time.Second)

el, _ := page.Element("#play-button")
box := el.MustShape().Box()
page.Mouse.MustMoveTo(box.X+box.Width/2, box.Y+box.Height/2)
page.Mouse.MustDown("left")
page.Mouse.MustUp("left")
fmt.Printf("Clicked at %.0f, %.0f\n", box.X+box.Width/2, box.Y+box.Height/2)

time.Sleep(5 * time.Second)
// Check if video appeared
videos, _ := page.Elements("video")
fmt.Printf("Videos after click: %d\n", len(videos))
```

If the click doesn't work:
- Check `pointer-events` CSS property (`window.getComputedStyle(el).pointerEvents`)
- If `none`, the parent element likely has the click handler — click parent
  or use `el.Eval('() => this.parentElement.click()')`
- Check if element is in viewport (positive x,y coordinates from `MustShape().Box()`)
- Check for overlays (cookie consent, ad blockers) that need closing first

## Step 4: Write the Lua Script

```lua
-- {domain}.lua
-- Description of how this site works

match_patterns = {
  "example.com/*",
  "*.example.com/*",
}

-- URLs containing these substrings are blocked
block_urls = {
  "disable-devtool",
  -- add other anti-bot scripts found during probe
}

-- Intercept JSON APIs that contain the HLS URL
intercepts = {
  {
    pattern = "/api/video",
    handler = function(url, body)
      local data = json_decode(body)
      if data and data.url then
        return data.url
      end
      return nil
    end
  }
}

function on_ready()
  wait_element("#play-button", 10)
  scroll_into_view("#play-button")
  sleep(1)
  click("#play-button")
end
```

## Step 5: Test the Script

Save the script to the config scripts directory (printed by the app on startup,
typically `~/Library/Application Support/wails-cast/scripts/`).

Run the extraction test:

```go
package main

import (
	"fmt"
	"wails-cast/pkg/extractor"
)

func main() {
	result, err := extractor.Extract("https://example.com/video-page")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Title: %s\n", result.Title)
	fmt.Printf("URL: %s\n", result.URL)
	fmt.Printf("Manifest: %d bytes\n", len(result.Manifest))
	if len(result.Manifest) > 0 {
		fmt.Printf("First line: %s\n", result.Manifest[:50])
	}
}
```

Run with: `go run /tmp/test_{site}.go`

## Step 6: Debug Failures

**"No Lua script found"**
- Check the script file exists in the config scripts directory
- Check `match_patterns` pattern matches the URL (use `wildcardMatch` logic)
- Pattern `example.com/*` matches `https://example.com/path` via host+path matching

**"timeout waiting for HLS URL"**
- The click didn't start video playback
- Re-run the probe with a longer wait after click
- Check if there are multiple click targets needed
- Check if the iframe needs to be clicked separately

**"invalid playlist: missing #EXTM3U header"**
- The manifest fetch returned something other than HLS
- Check the referer header (should be the HLS URL's origin)
- Check if the HLS URL expired between intercept and fetch

## Available Lua Functions Reference

| Function | Purpose |
|----------|---------|
| `wait_element(selector, timeout)` | Wait for element to appear |
| `click(selector)` | Trusted mouse click |
| `scroll_into_view(selector)` | Scroll element into viewport |
| `sleep(seconds)` | Wait |
| `log(message)` | Print debug message |
| `js(code)` | Evaluate JavaScript |
| `json_decode(string)` | Parse JSON to Lua table |

## Common Patterns

**Play button with pointer-events:none:**
```lua
function on_ready()
  wait_element("#play-video", 10)
  scroll_into_view("#play-video")
  sleep(1)
  -- Click parent which has the real handler
  js([[() => document.querySelector('#play-video').parentElement.click()]])
end
```

**Click inside iframe:**
```lua
function on_ready()
  wait_element("iframe", 10)
  scroll_into_view("iframe")
  sleep(2)
  click("iframe")
end
```

**Close cookie/ad overlay first:**
```lua
function on_ready()
  js([[() => {
    var close = document.querySelector('[class*="close"], [id*="close"], .cookie-btn');
    if (close) close.click();
  }]])
  wait_element("#play-video", 10)
  scroll_into_view("#play-video")
  sleep(1)
  click("#play-video")
end
```
