#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# shellcheck source=./lib/logging.sh
source "$DIR/lib/logging.sh"

level="$1"
shift

"$level" "$@"
