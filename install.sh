#!/usr/bin/env bash
# Build wails-cast and install it into /Applications.
set -euo pipefail

APP_NAME="wails-cast.app"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILT_APP="$SCRIPT_DIR/build/bin/$APP_NAME"
DEST="/Applications/$APP_NAME"

cd "$SCRIPT_DIR"

echo "==> Quitting running app (if any)"
pkill -x wails-cast 2>/dev/null && echo "    stopped running instance" || echo "    not running"

echo "==> Building (wails build)"
wails build

if [ ! -d "$BUILT_APP" ]; then
  echo "ERROR: expected build output not found at $BUILT_APP" >&2
  exit 1
fi

echo "==> Installing to $DEST"
rm -rf "$DEST"
cp -R "$BUILT_APP" "$DEST"

echo "==> Done. Installed $APP_NAME to /Applications"
