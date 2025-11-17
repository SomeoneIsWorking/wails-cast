# Wails-Cast Implementation Notes

## Project Overview
A desktop application built with Wails (Go + Vue.js) to cast local video files with subtitles to Chromecast devices.

## Key Requirements
- Cast local MKV/MP4 files to Chromecast
- Burn-in SRT subtitles during streaming
- Support seeking from both Vue interface and TV remote
- Display proper duration/timeline on Chromecast

---

## Implementation Attempts & Findings

### 1. Direct Matroska Pipe Streaming (Initial Approach)
**Implementation:**
- FFmpeg transcodes to Matroska format (container that supports h264+aac)
- Output piped directly to HTTP response: `ffmpeg ... -f matroska pipe:1`
- Chromecast receives stream via HTTP

**Configuration:**
```bash
ffmpeg -i input.mkv \
  -c:v libx264 -preset ultrafast -pix_fmt yuv420p -profile:v main \
  -vf "subtitles=input.srt:force_style='FontSize=24'" \
  -c:a aac -b:a 192k \
  -f matroska pipe:1
```

**Results:**
✅ **Working:**
- Playback successful
- Subtitles burned in correctly
- Seeking from Vue interface works (re-casts with `-ss` seek time)

❌ **Not Working:**
- **TV remote seeking fails** - causes infinite loading or jumps back to 00:00
- Duration shows but seeking is not functional

**Why TV Seeking Failed:**
- Matroska over pipe is not seekable (linear stream)
- `-live 1` flag was tried but doesn't make pipes truly seekable
- Chromecast expects seekable streams for timeline scrubbing

**Key Learnings:**
- Direct pipe streaming is simplest but doesn't support bidirectional seeking
- Chromecast distinguishes between seekable and non-seekable streams
- The `-live 1` flag adds duration metadata but doesn't enable seeking in pipes

---

### 2. HLS (HTTP Live Streaming) - Default Media Receiver Attempt
**Implementation:**
- FFmpeg generates HLS playlist (.m3u8) and segments (.ts files)
- Segments stored in temp directory
- Chromecast loads .m3u8 URL

**Configuration:**
```bash
ffmpeg -i input.mkv \
  -c:v libx264 -preset veryfast -pix_fmt yuv420p -profile:v main -g 48 \
  -vf "subtitles=input.srt:force_style='FontSize=24'" \
  -c:a aac -b:a 192k -ac 2 \
  -f hls -hls_time 4 -hls_list_size 0 \
  -hls_segment_filename segment%d.ts \
  -hls_base_url "http://192.168.1.22:8888/" \
  playlist.m3u8
```

**Initial Results:**
❌ **Not Working with Default Media Receiver:**
- Cast command succeeded but Chromecast never requested the stream
- No HTTP requests to playlist or segments
- TV showed loading screen indefinitely

**Attempts to Fix:**
1. **CORS Headers** (from Reddit suggestion)
   - Added `Access-Control-Allow-Origin: *` to playlist and segment responses
   - Result: No change, still no requests

2. **Content-Type Changes**
   - Tried `application/x-mpegURL` vs `application/vnd.apple.mpegurl`
   - Result: No change

3. **Absolute URLs in Playlist**
   - Added `-hls_base_url` to generate full URLs in playlist
   - Result: No change

**Critical Discovery:**
From Google Cast documentation (https://developers.google.com/cast/docs/media):
> "HTTP Live Streaming (HLS) - These are available through use of the Web Receiver SDK"

**This means:**
- Default Media Receiver does NOT support HLS natively
- HLS requires a **Custom Web Receiver** application
- OR using Shaka Player integration

---

### 3. HLS with Shaka Player - Auto Mode (FFmpeg generates all segments)
**Implementation:**
- FFmpeg generates all HLS segments upfront in one process
- Playlist updated as segments are created
- Modified go-chromecast library to add `useShakaForHls` custom data
- Shaka Player is built into Default Media Receiver but needs to be explicitly enabled

**Code Changes in go-chromecast:**

`application/application.go`:
```go
type LoadOptions struct {
    StartTime      int
    ContentType    string
    Transcode      bool
    Detach         bool
    ForceDetach    bool
    Duration       float32
    UseShakaForHls bool // NEW: Enable Shaka Player for HLS
}

// In play() method:
var customData interface{}
if opts.UseShakaForHls {
    customData = map[string]interface{}{
        "useShakaForHls": true,
    }
}

a.sendMediaRecv(&cast.LoadMediaCommand{
    PayloadHeader: cast.LoadHeader,
    CurrentTime:   opts.StartTime,
    Autoplay:      true,
    CustomData:    customData, // Pass to Chromecast
    Media: cast.MediaItem{
        ContentId:   mi.contentURL,
        StreamType:  "BUFFERED",
        ContentType: mi.contentType,
        Duration:    mi.duration,
    },
})
```

**Usage in wails-cast:**
```go
err = app.Load(mediaURL, application.LoadOptions{
    StartTime:      seekTime,
    ContentType:    "application/x-mpegURL",
    UseShakaForHls: true, // Enable Shaka Player
    Duration:       float32(duration),
})
```

**Results:**
✅ **Working:**
- Chromecast requests playlist and segments
- Playback works perfectly
- **TV remote seeking works!**
- Vue interface seeking works
- Subtitles burned in correctly

❌ **Issues:**
- **Duration display incorrect** - Shows wrong length (e.g., 5:29 instead of 2:09:17)
- FFmpeg generates playlist progressively, doesn't add `#EXT-X-ENDLIST` until complete
- Chromecast doesn't know full duration until all segments generated
- High CPU usage (transcoding entire file)
- Large disk usage (all segments stored)

**HTTP Request Pattern:**
```
GET /media.m3u8 (playlist request)
GET /segment0.ts
GET /segment1.ts
GET /segment2.ts
... (continues as playback progresses)
GET /media.m3u8 (periodic playlist refresh)
```

---

### 4. HLS Manual Mode - On-Demand Segment Generation (FINAL BEST SOLUTION) ✨
**Implementation:**
- **Generate complete playlist upfront** with all segment entries and correct duration
- **Transcode segments on-demand** only when Chromecast requests them
- Each segment is a separate FFmpeg process with `-ss` seek to specific timecode
- Smart connection checking prevents wasted CPU on cancelled seeks

**Key Innovation:**
Instead of FFmpeg generating the playlist, we calculate it ourselves:
```go
// Calculate total number of segments
numSegments := int(duration / segmentSize)

// Generate complete playlist with #EXT-X-ENDLIST
playlist := "#EXTM3U\n"
playlist += "#EXT-X-VERSION:3\n"
playlist += "#EXT-X-TARGETDURATION:5\n"
playlist += "#EXT-X-MEDIA-SEQUENCE:0\n"
playlist += "#EXT-X-PLAYLIST-TYPE:VOD\n"

for i := 0; i < numSegments; i++ {
    segmentDuration := 4.0 // Last segment may be shorter
    if i == numSegments-1 {
        segmentDuration = duration - float64(i*4)
    }
    playlist += fmt.Sprintf("#EXTINF:%.6f,\n", segmentDuration)
    playlist += fmt.Sprintf("http://IP:8888/segment%d.ts\n", i)
}
playlist += "#EXT-X-ENDLIST\n"
```

**On-Demand Segment Generation:**
```go
// When segment requested, check if connection stays alive
select {
case <-r.Context().Done():
    // Client cancelled - don't transcode
    return
case <-time.After(100 * time.Millisecond):
    // Connection alive, proceed
}

// Transcode just this segment
startTime := segmentNum * 4
cmd := exec.CommandContext(r.Context(), "ffmpeg",
    "-ss", fmt.Sprintf("%.2f", startTime),
    "-t", "4",
    "-i", videoPath,
    "-c:v", "libx264", "-preset", "veryfast",
    "-vf", "subtitles=...",
    "-c:a", "aac", "-b:a", "192k",
    "-f", "mpegts",
    segmentPath)
```

**Results:**
✅ **Fully Working - All Issues Resolved:**
- ✅ Chromecast knows **full duration immediately** (correct 2:09:17 display)
- ✅ **TV remote seeking works perfectly**
- ✅ Vue interface seeking works
- ✅ Subtitles burned in correctly
- ✅ **Smart CPU usage** - only transcodes segments that are actually played
- ✅ **Rapid seeking optimized** - cancelled requests don't waste CPU
- ✅ **Lower disk usage** - only stores segments that were actually requested
- ✅ **Fast startup** - no waiting for FFmpeg to generate segments

**Performance Benefits:**
- **100ms connection check** prevents wasted transcoding when seeking rapidly
- Segments cached after first transcode (reused if seeking back)
- Context-aware cancellation stops FFmpeg if request cancelled
- Each segment transcodes in ~200-500ms (much faster than full file)

**HTTP Request Pattern (with rapid seeking):**
```
GET /media.m3u8
  → Returns complete playlist instantly
GET /segment0.ts
  → Transcodes segment 0 (200ms)
GET /segment1.ts
  → Transcodes segment 1 (200ms)
GET /segment10.ts
  → Connection cancelled after 50ms, skips transcode
GET /segment20.ts
  → Connection cancelled after 30ms, skips transcode
GET /segment32.ts
  → Starts transcode, cancelled mid-way, FFmpeg stopped
GET /segment50.ts
  → Connection stays alive 100ms+, transcodes segment 50
```

**Implementation Files:**
- `hls_auto.go` - Original FFmpeg-generates-all-segments approach
- `hls_manual.go` - **Recommended** on-demand segment generation

---

## Technical Details

### FFmpeg Parameters Explained
- `-c:v libx264` - H.264 video codec (Chromecast compatible)
- `-preset veryfast` - Balance between speed and quality
- `-pix_fmt yuv420p` - Pixel format required by Chromecast
- `-profile:v main` - H.264 Main profile for wide compatibility
- `-g 48` - GOP size, keyframe every 48 frames (~2 seconds at 24fps) for better seeking
- `-vf "subtitles=..."` - Burn-in subtitles video filter
- `-c:a aac -b:a 192k -ac 2` - AAC audio, 192kbps, stereo (Chromecast max 2 channels)
- `-f hls` - Output format: HLS
- `-hls_time 4` - 4-second segments
- `-hls_list_size 0` - Keep all segments in playlist
- `-hls_base_url` - Prepend base URL to segment names in playlist

### HLS Architecture
```
┌─────────────┐
│  FFmpeg     │ Transcodes MKV → HLS segments
│  Process    │ Outputs: playlist.m3u8 + segment*.ts
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│  HLS Manager    │ Manages transcode sessions
│  (Go)           │ Serves playlist + segments
└────────┬────────┘
         │ HTTP
         ▼
┌─────────────────┐
│  Chromecast     │ Downloads segments sequentially
│  (Shaka Player) │ Enables seeking via segment selection
└─────────────────┘
```

### Session Management
- Each video+seekTime combination creates unique session
- Session ID format: `{filename}_{seekTime}`
- Output directory: `/tmp/wails-cast-hls/{sessionID}/`
- Sessions persist across seeks (reuses existing segments when possible)

---

## Failed Experiments

### 1. `-readrate` Flag
**Tried:** `ffmpeg -readrate 1 -i input.mkv ...`
**Purpose:** Encode at 1x playback speed to prevent overwhelming connection
**Result:** Made seeking worse - FFmpeg had to read for N seconds before outputting data at seek position N
**Lesson:** Don't throttle encoding speed when seeking is required

### 2. `-live 1` Flag with Matroska
**Tried:** `ffmpeg ... -f matroska -live 1 pipe:1`
**Purpose:** Enable "live streaming mode" for seekable streaming
**Result:** Duration metadata added, but pipe still not seekable
**Lesson:** Flags can't make a pipe truly seekable; need actual file-based access

### 3. Different Matroska Content Types
**Tried:** 
- `video/x-matroska`
- `video/mkv`
**Result:** No difference in seeking capability
**Lesson:** Content-Type doesn't affect seekability of piped content

### 4. HLS without Shaka Player
**Tried:** Cast .m3u8 URL to Default Media Receiver without custom data
**Result:** Chromecast accepted command but never requested stream
**Lesson:** Default Media Receiver needs explicit Shaka Player enablement for HLS

---

## Chromecast Compatibility

### Supported Media Formats (Default Media Receiver)
- **Containers:** MP4, WebM
- **Video Codecs:** H.264 (High Profile, level 4.1), VP8, VP9
- **Audio Codecs:** AAC (LC, HE), MP3, Opus, Vorbis
- **Streaming:** Progressive download ONLY by default

### HLS Support Requirements
- **Requires:** Web Receiver SDK OR Shaka Player integration
- **Custom Data:** `{"useShakaForHls": true}` must be passed in LoadMediaCommand
- **Playlist Format:** Standard HLS (.m3u8)
- **Segment Format:** MPEG-TS (.ts files)
- **CORS:** Must include proper CORS headers on all responses

### Tested On
- Device: Chromecast (3rd Gen)
- IP: 192.168.1.21:8009
- App: Default Media Receiver

---

## Current Limitations

### 1. Subtitle Burning
- Subtitles are burned into video (not selectable)
- Increases CPU usage during transcoding
- Cannot toggle subtitles on/off during playback

**Alternative (not implemented):** Use WebVTT subtitles as separate track
- Would require Custom Web Receiver
- Chromecast supports CEA-608/708, TTML, WebVTT

### 2. Encoding Performance
- Real-time transcoding required
- CPU intensive (especially with subtitle burning)
- `veryfast` preset balances quality vs speed
- Large files may struggle on slower machines

### 3. Network Bandwidth
- HLS generates many small HTTP requests
- 4-second segments = 900 requests for 1-hour video
- Each segment ~1-3MB depending on bitrate
- Total bandwidth similar to direct streaming but more overhead

### 4. Storage Usage
- Segments stored in /tmp during playback
- Not cleaned up automatically (process exit clears)
- Long videos generate many segment files
- Example: 2-hour movie = ~1800 segment files

### 5. First-Play Latency

**Auto Mode:**
- FFmpeg startup: ~200-600ms
- First segment generation: ~2-5 seconds
- Total time to first frame: ~3-6 seconds
- Subsequent seeks faster (segments may exist)

**Manual Mode (Recommended):**
- Playlist generation: <10ms (instant)
- First segment on-demand: ~200-500ms
- Total time to first frame: ~300-600ms
- **Subsequent seeks:** Similar ~200-500ms per segment
- **Rapid seeking:** <100ms (requests cancelled, no transcode)

---

## Code Architecture

### File Structure
```
wails-cast/
├── app.go              # Main Wails app, binds Go↔Vue
├── server.go           # HTTP server for media streaming
├── hls_auto.go         # HLS Auto Mode - FFmpeg generates all segments
├── hls_manual.go       # HLS Manual Mode - On-demand segments (RECOMMENDED)
├── chromecast.go       # Chromecast device communication
├── discovery.go        # mDNS device discovery
├── logger.go           # Structured logging
└── frontend/
    └── src/
        ├── services/
        │   ├── device.ts   # Device discovery/connection
        │   ├── file.ts     # File selection
        │   └── media.ts    # Playback control
        └── stores/
            └── cast.ts     # State management
```

### Key Components

**HLSManagerManual** (`hls_manual.go`) - **RECOMMENDED**
- Generates complete playlist upfront with full duration
- Transcodes segments on-demand when requested
- Smart connection checking prevents wasted CPU
- Context-aware cancellation for rapid seeking
- Caches transcoded segments for reuse

**HLSManagerAuto** (`hls_auto.go`) - Alternative approach
- Single FFmpeg process generates all segments
- Progressive playlist updates
- Higher CPU and disk usage
- Duration issues until completion

**Server** (`server.go`)
- Routes requests to HLS manager or direct file serving
- Tracks current media file and subtitle
- Manages seek time state
- Determines transcode vs direct serve

**App** (`app.go`)
- Wails application entry point
- Exposes Go functions to Vue frontend
- Manages playback state
- Handles file dialogs
- Coordinates server, HLS, and Chromecast components

---

## Environment Details

### Dependencies
- **Wails:** v2.10.2
- **Go:** 1.21+
- **FFmpeg:** Required, must be in PATH
  - Used for transcoding and probing
  - `ffprobe` for duration detection
- **go-chromecast:** Modified local copy
  - Added `UseShakaForHls` support
  - Fork from: https://github.com/vishen/go-chromecast

### Network Setup
- Server IP: 192.168.1.22:8888
- Chromecast IP: 192.168.1.21:8009
- mDNS discovery for device detection
- Local HTTP server for media streaming

---

## Best Practices Discovered

### 1. HLS Segment Duration
- **4 seconds** found to be optimal balance
- Too short: Many HTTP requests, overhead
- Too long: Slow seeking, large memory buffers

### 2. FFmpeg Preset
- `veryfast` preset chosen for real-time performance
- Slower presets (medium, slow) cause buffering
- Faster presets (ultrafast, superfast) reduce quality noticeably

### 3. Keyframe Interval
- `-g 48` (every 2 seconds at 24fps)
- Enables precise seeking to ~2-second granularity
- Smaller values increase file size unnecessarily
- Larger values make seeking less precise

### 4. Session Management
- Reuse sessions for same video+seek position
- Clean up sessions on app exit
- Don't prematurely kill FFmpeg (let segments complete)

### 5. CORS Headers
- Required on ALL responses (playlist + segments)
- `Access-Control-Allow-Origin: *` for development
- Production should restrict to Chromecast IPs

---

## Debugging Tips

### Check if Chromecast is Requesting Stream
```bash
# Watch server logs for HTTP requests
# Should see:
# - GET /media.m3u8 (playlist)
# - GET /segment*.ts (segments)
```

### Verify HLS Playlist Format
```bash
cat /tmp/wails-cast-hls/matrix.mkv_0/playlist.m3u8

# Should contain absolute URLs:
# http://192.168.1.22:8888/segment0.ts
# NOT relative paths:
# segment0.ts
```

### Test HLS Stream Locally
```bash
# Use VLC or ffplay to test stream
ffplay http://192.168.1.22:8888/media.m3u8
```

### Check FFmpeg Process
```bash
# Verify FFmpeg is running
ps aux | grep ffmpeg

# Check segment files being created
ls -lah /tmp/wails-cast-hls/matrix.mkv_0/
```

### Test Without Wails
```bash
# Use test harness with manual mode (recommended)
go run test_hls.go logger.go discovery.go server.go hls_manual.go chromecast.go app.go

# Or with auto mode
go run test_hls.go logger.go discovery.go server.go hls_auto.go chromecast.go app.go
```

### Watch On-Demand Segment Generation
```bash
# Check which segments have been transcoded
ls -lah /tmp/wails-cast-hls/matrix.mkv_0/

# You'll see only segments that were actually requested
# Example after seeking around:
# segment0.ts  segment1.ts  segment2.ts  segment50.ts  segment120.ts
# (Not all 1940 segments!)
```

---

## Future Improvements

### Potential Enhancements
1. **WebVTT Subtitle Track** - Selectable subtitles instead of burn-in
2. **Quality Selection** - Multiple bitrate variants for adaptive streaming
3. **Session Cleanup** - Automatic removal of old segment files
4. **Encoding Cache** - Reuse segments for repeated plays
5. **Hardware Acceleration** - Use GPU for faster encoding (`-hwaccel`)
6. **Resume Playback** - Remember position across app restarts
7. **Playlist Queue** - Cast multiple videos in sequence
8. **Audio Track Selection** - Choose from multiple audio tracks
9. **Direct Play Detection** - Skip transcoding for compatible files
10. **Progress Reporting** - Show encoding progress in UI

### Known Issues
- No automatic session cleanup (relies on process exit)
- Large temp directory usage for long videos
- No encoding progress indicator
- Subtitle font size hardcoded (FontSize=24)

---

## References

### Documentation
- [Google Cast Media Documentation](https://developers.google.com/cast/docs/media)
- [HLS Specification](https://datatracker.ietf.org/doc/html/rfc8216)
- [FFmpeg HLS Documentation](https://ffmpeg.org/ffmpeg-formats.html#hls-2)
- [Shaka Player](https://github.com/google/shaka-player)

### Key Reddit/Forum Findings
1. Chromecast requires CORS headers for HLS (source: Reddit)
2. `useShakaForHls` custom data enables HLS in Default Media Receiver
3. Content-Type must be `application/x-mpegURL` for HLS recognition

### Related Projects
- [plex](https://www.plex.tv/) - Full media server with transcoding
- [jellyfin](https://jellyfin.org/) - Open source media server
- [videostream](https://getvideostream.com/) - Chrome extension for casting

---

## Conclusion

**Final Best Solution: HLS Manual Mode (On-Demand Segments)**
- Generate complete playlist upfront with accurate duration
- Transcode segments only when actually needed
- Smart cancellation prevents wasted CPU during seeking
- Proper duration display on Chromecast (e.g., 2:09:17)
- Full seeking support from TV remote and app interface
- Subtitles burned in during per-segment transcode
- Optimal resource usage (CPU, disk, memory)

**Key Insights:**
1. **`useShakaForHls: true`** custom data enables HLS in Default Media Receiver
2. **Manual playlist generation** solves duration display issues instantly
3. **On-demand transcoding** dramatically improves efficiency vs. upfront generation
4. **100ms connection check** prevents wasted work during rapid seeking
5. **Context-aware FFmpeg** allows proper cancellation of in-progress transcodes

**Architecture Comparison:**
```
Auto Mode:  FFmpeg → (all segments) → Disk → Chromecast
            ⚠️ High CPU, Disk usage, Duration issues

Manual Mode: Chromecast requests → Server generates → FFmpeg (one segment) → Cache
             ✅ Smart CPU, Minimal disk, Instant duration, Cancel-aware
```

**Total Development Time:** ~10 hours of experimentation
**Lines of Code:** ~1000 (Go backend + TypeScript frontend)
**Success Rate:** 4th major approach perfected

---

*Last Updated: November 17, 2025*
*Tested with: Chromecast 3rd Gen, Wails v2.10.2, FFmpeg 6.0*
*Recommended: HLS Manual Mode (`hls_manual.go`)*
