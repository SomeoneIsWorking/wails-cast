# Audio/Video Desync Fixes for Demuxed HLS Streams

## Problem
When serving HLS streams with **separate audio and video segments** that have **different durations**, Shaka Player can experience desynchronization. This manifests as:
- Audio and video playing out of sync
- Sync issues that are temporarily fixed by seeking forward/backward
- Playback stalls or buffering issues

## Root Cause
The desync occurs because:
1. Audio and video segments have different durations (e.g., video: 8s, audio: 6s)
2. Shaka Player expects segments to be time-aligned
3. Without proper timestamp information, the player can't accurately synchronize the streams

## Solutions Implemented

### 1. FFmpeg Timestamp Normalization (`pkg/hls/ffmpeg.go`)

Added the following flags to ensure proper timestamp handling:

```go
"-avoid_negative_ts", "make_zero",  // Normalize timestamps to start at zero
"-start_at_zero",                    // Ensure timestamps start at zero
"-vsync", "cfr",                     // Constant frame rate for consistent timing
"-muxdelay", "0",                    // Minimize muxing delay
"-muxpreload", "0",                  // Minimize muxing preload
```

**Why this helps:**
- Ensures all segments have consistent, normalized timestamps
- Prevents negative timestamps that can confuse players
- Maintains constant frame rate for predictable timing

### 2. EXT-X-PROGRAM-DATE-TIME Tags (`pkg/stream/remote.go` & `pkg/stream/local.go`)

Added program date time tags to each segment in the playlist:

```go
baseTime := time.Now()
cumulativeTime := 0.0

for i := range playlist.Segments {
    segment := &playlist.Segments[i]
    segmentTime := baseTime.Add(time.Duration(cumulativeTime * float64(time.Second)))
    segment.ProgramDateTime = segmentTime.Format(time.RFC3339Nano)
    cumulativeTime += segment.Duration
}
```

**Why this helps:**
- Provides absolute timestamps for each segment
- Allows Shaka Player to synchronize audio and video based on wall-clock time
- Even if segment durations differ, the player can align them correctly

### 3. Shaka Player Configuration (`CastReceiver/js/receiver.js`)

Added specific Shaka Player configuration for demuxed streams:

```javascript
playbackConfig.shakaConfig = {
  streaming: {
    rebufferingGoal: 2,        // Smoother playback with more buffering
    stallEnabled: true,         // Detect and handle stalls
    alwaysStreamText: true,     // Consistent subtitle handling
    forceTransmuxTS: true,      // Proper MPEG-TS handling
  },
  manifest: {
    defaultPresentationDelay: 10,  // Allow for segment duration variance
  }
};
```

**Why this helps:**
- `rebufferingGoal`: Maintains a larger buffer to handle timing variations
- `stallEnabled`: Detects when playback stalls and attempts recovery
- `forceTransmuxTS`: Ensures MPEG-TS segments are properly transmuxed
- `defaultPresentationDelay`: Allows for slight timing differences between segments

## Additional Recommendations

### If Issues Persist

1. **Ensure Segment Duration Consistency**
   - Try to make audio and video segments have the same target duration
   - Use FFmpeg's `-force_key_frames` to align segment boundaries:
   ```bash
   -force_key_frames "expr:gte(t,n_forced*8)"  # Force keyframes every 8 seconds
   ```

2. **Check Segment Alignment**
   - Verify that audio and video segments start at the same timestamps
   - Use `ffprobe` to inspect segment timing:
   ```bash
   ffprobe -show_packets -select_streams v:0 segment.ts
   ```

3. **Monitor Shaka Player Logs**
   - Enable debug mode: `?debug=true` in the receiver URL
   - Look for warnings about:
     - "Segments do not start at the same time"
     - "Large gap in timestamps"
     - "Buffering stall detected"

4. **Consider Using Muxed Streams**
   - If desync persists, consider muxing audio and video into single segments
   - This eliminates timing issues but increases bandwidth usage

5. **Adjust Segment Duration**
   - Try using a consistent segment duration for both audio and video
   - Common values: 2s, 4s, 6s, 8s, or 10s
   - Shorter segments = better seeking, but more overhead
   - Longer segments = less overhead, but slower seeking

## Testing the Fixes

1. **Clear the cache** to ensure old segments are regenerated:
   ```bash
   rm -rf /tmp/wails-cast-hls/*
   ```

2. **Test with a problematic stream** that previously had desync issues

3. **Monitor for:**
   - Smooth playback without audio/video drift
   - No desync after seeking
   - Consistent behavior across different content

4. **Check the generated playlists** to verify:
   - EXT-X-PROGRAM-DATE-TIME tags are present
   - Segment durations are reasonable
   - Timestamps are sequential

## Technical Details

### How EXT-X-PROGRAM-DATE-TIME Works

```m3u8
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:8
#EXTINF:8.0,
#EXT-X-PROGRAM-DATE-TIME:2025-11-30T15:54:07.000000000+03:00
/video_0/segment_0.ts
#EXTINF:8.0,
#EXT-X-PROGRAM-DATE-TIME:2025-11-30T15:54:15.000000000+03:00
/video_0/segment_1.ts
```

The player uses these timestamps to:
1. Calculate the exact playback time for each segment
2. Synchronize audio and video based on wall-clock time
3. Handle segments with different durations gracefully

### FFmpeg Timestamp Handling

The flags we added ensure:
- **`-avoid_negative_ts make_zero`**: Shifts all timestamps so the earliest is 0
- **`-start_at_zero`**: Forces the first frame to have timestamp 0
- **`-vsync cfr`**: Maintains constant frame rate (no dropped/duplicated frames)
- **`-muxdelay 0`**: No delay when muxing audio/video
- **`-muxpreload 0`**: No preload buffer (immediate muxing)

This creates segments with clean, predictable timestamps that players can easily synchronize.

## Conclusion

The combination of:
1. Proper FFmpeg timestamp handling
2. EXT-X-PROGRAM-DATE-TIME tags
3. Shaka Player configuration

Should resolve most audio/video desync issues with demuxed HLS streams. If problems persist, consider the additional recommendations above or reach out for further debugging.
