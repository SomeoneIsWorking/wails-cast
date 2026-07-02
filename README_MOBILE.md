# wails-cast mobile companion

A Fyne v2 remote-control app for a wails-cast instance running on the LAN.
It consumes the shared `pkg/castapi` client (mDNS discovery + HTTP API) that
the desktop app also uses internally.

## Build & run

Desktop preview (the fastest way to iterate on the UI):

```
go run ./cmd/mobile
```

Package for Android — must be run from `cmd/mobile/`:

```
cd cmd/mobile
fyne package -os android -appID io.wailscast.remote
```

Requires:
- `fyne` CLI: `go install fyne.io/tools/cmd/fyne@latest`
- Android SDK + NDK. On macOS with Homebrew:
  ```
  brew install --cask android-commandlinetools
  export ANDROID_HOME=/opt/homebrew/share/android-commandlinetools
  yes | sdkmanager --sdk_root="$ANDROID_HOME" --licenses
  sdkmanager --sdk_root="$ANDROID_HOME" --install \
    "platform-tools" "platforms;android-34" \
    "build-tools;34.0.0" "ndk;29.0.14206865"
  ```
- Export env before running `fyne package`:
  ```
  export ANDROID_HOME=/opt/homebrew/share/android-commandlinetools
  export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/$(ls $ANDROID_HOME/ndk | tail -1)"
  ```

The produced APK is debug-signed. For a release build, pass
`-release` and sign the output with `apksigner`.

Package for iOS (macOS + Xcode required):

```
cd cmd/mobile
fyne package -os ios -appID io.wailscast.remote
```

`Icon.png` in `cmd/mobile/` is a placeholder; replace it with a proper 1024×1024
launcher icon before shipping.

## Screens

- Connect — discover instances over mDNS, or connect manually by host/port/token.
  The last-used base URL and token are stored in Fyne preferences.
- Library — pick an output device, browse the remote library, pick tracks
  and quality, then play.
- Now Playing — poll transport state every second; play/pause/stop, skip
  buttons, seek slider, volume slider, mute toggle.

## Deferred (not yet on mobile)

- Torrent management (add-magnet works via the API but no UI yet)
- Subtitle style / sync live controls
- Library organize / identify / TMDB flows
- Season-batch translation flow
