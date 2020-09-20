#!/usr/bin/env bash
# Wrap goimports for VSCode
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

args=("-local=$(grep module "$DIR/../go.mod" | awk '{ print $2 }')" "$@")

set -x
exec "$DIR/gobin.sh" golang.org/x/tools/cmd/goimports "${args[@]}"
