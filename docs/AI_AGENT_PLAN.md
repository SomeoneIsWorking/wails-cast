# AI Agent for Script Generation — Plan

An AI agent that automatically creates Lua extraction scripts by
probing a website and writing the script based on what it discovers.

## Workflow

```
User enters URL
       │
       ▼
┌──────────────────┐
│ 1. Probe Phase   │
│ - Open headful    │
│   browser (rod)   │
│ - Navigate to URL │
│ - Log all network │
│   requests        │
│ - Take screenshot │
│ - Get page HTML   │
│ - List iframes,   │
│   video elements, │
│   play buttons    │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 2. Interaction   │
│    Phase         │
│ (AI-driven loop) │
│                   │
│ For each possible │
│ action:           │
│ - Click button    │
│ - Wait for change │
│ - Check for video │
│ - If fail, try    │
│   next action     │
│ - If succeed,     │
│   record the      │
│   sequence        │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ 3. Script Gen    │
│    Phase         │
│ - Analyze network │
│   logs            │
│ - Find HLS URL    │
│   source          │
│ - Write Lua       │
│   script          │
│ - Test script     │
│ - Return to user  │
└──────────────────┘
```

## Phase 1: Probe

The agent opens a headful browser (so user can see what's happening)
and collects:

- **Page title and URL**
- **All iframes** (id, class, src)
- **All video elements** (src, currentSrc)
- **Screenshot** of the viewport
- **Full page HTML** (or DOM tree)
- **All network requests** and their responses (status, content-type, URL)
- **Candidate click targets**: elements with id/class containing "play",
  "video", "player", "start", "watch"
- **Ad/overlay elements** that might need closing first

## Phase 2: Interaction (AI-Driven Loop)

The agent tries actions in sequence, checking for success after each:

1. Look for and close cookie consent / ad overlays
2. Scroll the player area into view
3. Click the most likely play button
4. If an iframe appears, try clicking inside it
5. If a JSON API is detected in network logs, set up an intercept
6. Check if HLS manifest appeared in network traffic

Success criteria: An HLS manifest (Content-Type: application/vnd.apple.mpegurl)
was detected, or a Lua `intercepts.handler` returned a URL.

## Phase 3: Script Generation

The agent writes the Lua script based on what it learned:

- `block_urls`: Any scripts that seemed to interfere (anti-bot, etc.)
- `intercepts`: Any JSON APIs that returned the HLS URL
- `on_ready()`: The sequence of actions that successfully started playback

The generated script is saved and tested automatically.

## AI Integration Points

The agent can use an LLM API (Gemini, OpenAI, etc.) to:

- **Analyze page HTML** to find the player and play button
- **Decide what to click** based on element content and structure
- **Parse obfuscated JavaScript** to find video URLs
- **Generate the Lua script** from the observed behavior

The app already has Gemini integration (see `pkg/ai/`). The same
infrastructure can be reused for the agent.

## Backend API

```go
// AIScriptGenerate calls the AI agent to probe a URL and generate a script.
// The agent runs in a goroutine and emits progress events.
func (a *App) AIScriptGenerate(targetURL string) (string, error)
    // Returns the generated script content for user review.

// AIScriptProgress returns the current progress of a running generation.
func (a *App) AIScriptProgress(taskID string) (*ScriptGenerationProgress, error)

// AIScriptCancel cancels a running generation.
func (a *App) AIScriptCancel(taskID string) error
```

## Events (Frontend Communication)

| Event | Payload | Description |
|-------|---------|-------------|
| `ai:script:progress` | `{taskID, step, message}` | Current step |
| `ai:script:complete` | `{taskID, script}` | Script generated |
| `ai:script:error` | `{taskID, error}` | Error occurred |

## Implementation Steps

1. Create `pkg/agent/` package with the probing and interaction logic
2. Integrate with the existing Gemini API in `pkg/ai/`
3. Add the AI generation API endpoints in `app.go`
4. Add frontend progress UI
5. Connect to the Lua Manager panel's "Generate with AI" button
