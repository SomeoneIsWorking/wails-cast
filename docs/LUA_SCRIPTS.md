# Lua Scripts Guide

This document explains how to create Lua extraction scripts for websites.
Scripts are stored in the application config directory under `scripts/`.

Each script declares which URLs it handles via the `match_patterns` table.
The extractor scans all `.lua` files and checks `match_patterns` to find
the right script for a given URL.

## Script Structure

```lua
-- Websites this script applies to (supports * wildcards)
match_patterns = {
  "example.com/*",
  "sub.example.com/*",
}

-- Optional: URLs containing these substrings will be blocked
-- (their response is replaced with an empty JS file)
block_urls = {
  "some-script.js",
  "another-tracker",
}

-- Optional: URLs matching these patterns are intercepted.
-- The response body is passed to `handler` which should return
-- the HLS manifest URL, or nil.
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

-- Called after the page loads. Use this to interact with the page
-- (click play buttons, close overlays, wait for elements).
function on_ready()
  wait_element("#play-button", 15)
  scroll_into_view("#play-button")
  sleep(1)
  click("#play-button")
end
```

## Available Lua Functions

### `wait_element(selector, timeout)`
Wait for an element to appear. `selector` is a CSS selector.
`timeout` is max seconds to wait (default 10).

### `click(selector)`
Perform a trusted mouse click at the element's center.

### `scroll_into_view(selector)`
Scroll the page so the element is centered in the viewport.

### `sleep(seconds)`
Wait for N seconds.

### `log(message)`
Print a message (appears in the extractor console output).

### `js(code)`
Evaluate JavaScript on the page. The code should be a function
expression like `() => { ... }` so it gets called automatically.

### `json_decode(string)`
Parse JSON into a Lua table.

## How It Works

1. The extractor launches a **headless** browser
2. It blocks matching URLs in `block_urls`
3. It intercepts matching URLs in `intercepts`, fetches them with
   Go's HTTP client, and calls the Lua `handler` with the response body
4. A CDP listener watches all network responses for HLS Content-Type
5. It navigates to the page and calls `on_ready()`
6. It waits for an HLS URL to be found (from an intercept handler
   or the CDP listener)
7. It fetches the HLS manifest and returns

## HLS Detection

HLS manifests can be detected two ways:

- **CDP listener** (automatic): Catches any response with
  `Content-Type: application/vnd.apple.mpegurl` or `application/x-mpegURL`.
  No Lua code needed — just make the video start playing.

- **Intercept handler** (manual): Use when the HLS URL is embedded in
  a JSON or other response. The handler returns the URL string.

## Tips

- **Probe first**: Navigate the site manually (use a temporary Go script)
  to find selectors, click targets, and network patterns.
- **pointer-events:none**: Some sites use this CSS to block clicks.
  `click()` dispatches a trusted mouse event that bypasses it.
- **Scroll first**: Many players are below the fold.
  Always `scroll_into_view` before `click`.
- **Wait after click**: After clicking, the video may load in an iframe.
  The CDP listener will find it automatically.
