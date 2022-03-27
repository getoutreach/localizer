#!/usr/bin/env bash
# shell wrapper runs a shell script inside of the devbase
# repository, which contains shared shell scripts.
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
libDir="$DIR/../.bootstrap"

"$DIR/devbase.sh"

script="$1"

shift

exec "$libDir/shell/$script" "$@"
