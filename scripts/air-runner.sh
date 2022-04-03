#!/usr/bin/env bash
# This script enables us to run different bins with one Air configuration
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
binDir="$DIR/../bin"

echo "Running $DEV_CONTAINER_EXECUTABLE"
exec "$binDir/${DEV_CONTAINER_EXECUTABLE:-localizer}"
