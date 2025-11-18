# Wails Cast

A desktop application for streaming media to Chromecast devices using Wails and Go.

## Features

- Discover Chromecast devices on your network
- Stream local media files to Chromecast
- Manage playback with intuitive controls
- Support for HLS streaming (both automatic and manual modes)
- File explorer for easy media selection
- Real-time device status monitoring

## Prerequisites

- Go 1.21 or higher
- Node.js 16 or higher
- npm or yarn
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Installation

1. Clone the repository:
```bash
git clone https://github.com/SomeoneIsWorking/wails-cast.git
cd wails-cast
```

2. Install dependencies:
```bash
wails build
```

## Development

To run in live development mode with hot reload:

```bash
wails dev
```

This will start:
- A Vite development server for frontend hot reload
- The Go backend with live compilation
- A dev server on http://localhost:34115 for debugging

## Building

To build a production-ready application:

```bash
wails build
```

The built application will be located in the `build/bin/` directory.

## Project Structure

- `app.go` - Main application entry point
- `chromecast.go` - Chromecast device management
- `discovery.go` - Network device discovery
- `server.go` - HTTP server for media streaming
- `hls_*.go` - HLS streaming implementations
- `frontend/` - Vue.js frontend application
- `go-chromecast/` - Go Chromecast library

## Configuration

Edit `wails.json` to configure project settings. For more information, see the [Wails documentation](https://wails.io/docs/reference/project-config).

## License

MIT License - see LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
