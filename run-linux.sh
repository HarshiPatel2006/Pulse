#!/usr/bin/env bash
set -euo pipefail
PULSE_ROOT="$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")"
PULSE_BINARY="$PULSE_ROOT/dist/pulse-linux-amd64"
if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
    echo 'This launcher requires Linux x86_64.' >&2
    exit 1
fi
PULSE_REBUILD=0
[[ -x "$PULSE_BINARY" ]] || PULSE_REBUILD=1
if [[ "$PULSE_REBUILD" == 0 ]] && find "$PULSE_ROOT" -type f \( -name '*.go' -o -name go.mod -o -name go.sum \) -newer "$PULSE_BINARY" -print -quit | read -r _; then
    PULSE_REBUILD=1
fi
if [[ "$PULSE_REBUILD" == 1 ]]; then
    PULSE_GO="${GO:-}"
    if [[ -z "$PULSE_GO" ]]; then
        PULSE_GO="$(command -v go || true)"
        [[ -n "$PULSE_GO" ]] || PULSE_GO="$HOME/.local/go-pulse/bin/go"
    fi
    if [[ ! -x "$PULSE_GO" ]]; then
        echo 'Building requires Go 1.26 or newer. Install Go locally or set GO=/path/to/go.' >&2
        exit 1
    fi
    mkdir -p "$PULSE_ROOT/dist"
    ( cd "$PULSE_ROOT" && "$PULSE_GO" build -trimpath -ldflags='-s -w' -o "$PULSE_BINARY" . )
fi
exec "$PULSE_BINARY" "$@"
