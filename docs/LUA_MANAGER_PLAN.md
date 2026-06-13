# Lua Manager Panel — Plan

A UI panel in the application for managing Lua extraction scripts.

## Goals

- View all installed Lua scripts
- Enable/disable scripts
- Create new scripts from scratch
- Trigger AI agent to generate a script for a given URL
- Test a script against a URL

## UI Layout

```
┌──────────────────────────────────────────────┐
│  Lua Script Manager                           │
├──────────┬───────────────────────────────────┤
│ Scripts  │  [Script Content / Editor]        │
│ List     │                                   │
│          │  Domain: example.com              │
│ ┌──────┐ │  Status: Enabled                  │
│ │example│ │                                   │
│ │site2  │ │  block_urls = { ... }            │
│ │       │ │  function on_ready()             │
│ │       │ │    ...                            │
│ │       │ │  end                             │
│ │       │ │                                   │
│ └──────┘ │  [Save]  [Disable]  [Delete]      │
│          │                                   │
│ [+ New]  │  [Generate with AI ▾]             │
│          │                                   │
└──────────┴───────────────────────────────────┘
```

## Components

### Script List (left panel)
- Lists `.lua` files from `{configDir}/scripts/`
- Shows domain name, enabled/disabled status
- Click to select and view script

### Script Editor (right panel)
- Syntax-highlighted Lua editor (CodeMirror or similar)
- Domain field (read-only, from filename)
- Enable/disable toggle
- Save, Revert, Delete buttons

### AI Generation Dropdown
- "Generate Script from URL"
- Opens a dialog: "Enter URL for AI to analyze"
- Triggers the AI agent workflow

### Test Button
- "Test Script" — runs the extractor with this script and a test URL
- Shows extraction logs in a panel below
- Shows success/failure and manifest preview

## Data Model

```json
{
  "domain": "example.com",
  "filename": "example.com.lua",
  "enabled": true,
  "content": "-- Lua script content",
  "created": "2026-01-01T00:00:00Z",
  "updated": "2026-01-02T00:00:00Z"
}
```

Disabled scripts are renamed to `{domain}.lua.disabled` so the
extractor ignores them but the original is preserved.

## Go API (Backend)

```go
type App struct {
    // existing fields...
}

// ListScripts returns all scripts in the config directory.
func (a *App) ListScripts() ([]ScriptInfo, error)

// GetScript returns the content of a specific script.
func (a *App) GetScript(domain string) (string, error)

// SaveScript creates or updates a script file.
func (a *App) SaveScript(domain string, content string) error

// DeleteScript removes a script file.
func (a *App) DeleteScript(domain string) error

// ToggleScript enables/disables a script.
func (a *App) ToggleScript(domain string, enabled bool) error

// TestScript runs extraction with the script against the given URL
// and returns logs + result.
func (a *App) TestScript(domain string, testURL string) (*ScriptTestResult, error)

// GenerateScript triggers AI to create a script for a URL.
// Returns the partially-complete script for the user to review.
func (a *App) GenerateScript(targetURL string) (*ScriptGenerationResult, error)
```

## Implementation Steps

1. Add script list/edit API endpoints in `app.go`
2. Add a new frontend page/component for the Lua Manager
3. Implement the AI generation workflow
4. Add script testing functionality
