# Project state

## Comparison baseline

The baseline is manually combining local media browsing, download/extraction tools, transcoding,
subtitle utilities, and a separate Chromecast controller. Wails Cast intends one desktop workflow for
library management, media preparation, subtitles, casting, durable background work, and authenticated
companion control.

## Current focus

S003 is the current focus.

## Capability inventory

| ID | Capability or outcome | State | Factual dependency | Goals |
| --- | --- | --- | --- | --- |
| S001 | The desktop discovers Chromecast devices and controls play, pause, stop, seek, volume, and mute | partial | — | G001 |
| S002 | Local files, remote URLs, HLS streams, track selection, transcoding, and cache reuse form one playback pipeline | partial | S001 | G001 |
| S003 | Background downloads, extraction, translation, progress, cancellation, and resume preserve valid state | partial | S002 | G001 |
| S004 | Embedded, nearby, external, translated, and burned-in subtitle workflows are available per episode | partial | S002 | G001 |
| S005 | Library scanning, identification, organization, seasons, episodes, and torrent submission support media management | partial | S002 | G001 |
| S006 | A token-protected advertised Remote API exposes library, playback, subtitle, translation, and settings operations | partial | S001, S004, S005 | G001 |
| S007 | Chromecast devices are discovered automatically over mDNS | partial | — | G001 |
| S008 | Play, pause, stop, seek, volume, and mute control the selected Chromecast | partial | S007 | G001 |
| S009 | Local files can be probed, prepared, and cast | partial | S007 | G001 |
| S010 | Remote URLs and HLS streams can be extracted, prepared, and cast | partial | S007 | G001 |
| S011 | Unsupported media is transcoded and reusable results are cached | partial | S009, S010 | G001 |
| S012 | Downloads and extraction expose progress, cancellation, and resume without corrupting state | partial | S010 | G001 |
| S013 | Embedded, nearby, and external subtitle tracks can be selected per episode | partial | S009, S010 | G001 |
| S014 | Subtitles can be translated, synchronized, exported, or burned into video | partial | S013 | G001 |
| S015 | Shows are identified and organized into seasons and episodes in a local library | partial | S009 | G001 |
| S016 | Torrent submissions enter the same managed media workflow | partial | S015 | G001 |
| S017 | Remote API access is authenticated by token and advertised only through the configured scope | partial | S006 | G001 |

## Capability details

### S001 — Device control

The application implements mDNS discovery and Chromecast transport controls in its Go backend and Vue
frontend.

Gap: no current canonical integration suite records real-device discovery and full transport behavior.

### S002 — Media preparation and playback

Local probing, URL/HLS extraction, track choice, segmented download, transcoding, and cache reuse exist in
the production code.

Gap: the complete matrix of local, remote, stream, codec, and failure cases is not durably verified.

### S003 — Durable background work

Background downloading and translation expose progress, stop, resume, and cancellation behavior.

Gap: these owners are under active refactoring in the current worktree and need combined verification
before they can be called complete.

### S004 — Subtitle workflows

The product supports embedded, nearby, translated, and external subtitles, sizing and sync, burn-in,
closed-caption filtering, export, and translation.

Gap: end-to-end playback validation across each subtitle source and mode is not recorded.

### S005 — Media library

The code implements show/season/episode trees, TMDB identification, organization previews, batch
translation, remote library browsing, and torrent submission.

Gap: full-library migration, conflict, and partial-failure behavior remains unverified.

### S006 — Companion API

The configurable API uses token authentication, mDNS advertisement, and routes for the documented remote
operations consumed by WailsCast Remote.

Gap: the authentication and endpoint set lacks a canonical positive/negative protocol gate.

### S007 — Chromecast discovery

The production backend implements mDNS device discovery. Gap: current real-device discovery evidence
is not recorded.

### S008 — Chromecast transport controls

Play, pause, stop, seek, volume, and mute are implemented. Gap: the full control matrix lacks a
current real-device gate.

### S009 — Local-file casting

Local probing and media preparation feed the shared playback pipeline. Gap: representative codec and
failure cases remain unverified.

### S010 — URL and HLS casting

Remote URL and HLS extraction feed the shared playback pipeline. Gap: source, network, and recovery
coverage remains incomplete.

### S011 — Transcoding and cache reuse

Transcoding and reusable output caching are implemented. Gap: the codec matrix, invalidation, and
partial-failure behavior lack a canonical gate.

### S012 — Durable downloads

Downloads and extraction expose progress, cancellation, and resume. Gap: these owners are under active
refactoring and need combined verification before valid-state preservation is established.

### S013 — Subtitle sources

Embedded, nearby, and external subtitle selection is implemented per episode. Gap: every source path
has not been validated during real playback.

### S014 — Subtitle processing

Translation, synchronization, export, and burn-in are implemented. Gap: completion, cancellation, and
playback behavior remains unverified end to end.

### S015 — Organized media library

The application implements identification plus show, season, and episode organization. Gap: migration,
conflict, and partial-failure behavior remains unverified.

### S016 — Torrent submission

Torrent submission enters the managed media workflow. Gap: acquisition through playable completion
lacks a durable integration run.

### S017 — Remote API security and discovery

The configurable API uses token authentication and mDNS advertisement. Gap: positive and negative
protocol controls for scope, authentication, and the complete endpoint set are absent.
