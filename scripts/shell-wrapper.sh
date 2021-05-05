#!/usr/bin/env bash
# shell wrapper runs a shell script inside of the
# bootstrap-libs
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
libDir="$DIR/../.bootstrap"

"$DIR/bootstrap-lib.sh"

script="$1"

shift

exec "$libDir/shell/$script" "$@"
