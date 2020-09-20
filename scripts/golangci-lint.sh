#!/usr/bin/env bash
# Wrap golangci-lint for VSCode
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

exec "$DIR/gobin.sh" github.com/golangci/golangci-lint/cmd/golangci-lint "$@"
