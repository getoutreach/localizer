#!/usr/bin/env bash
# Wraps shfmt for usage with VSCode

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBIN="$DIR/gobin.sh"

# Always set simplify mode.
args=("-s" "$@")
exec "$GOBIN" "mvdan.cc/sh/v3/cmd/shfmt" "${args[@]}"
