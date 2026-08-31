# Project state

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
