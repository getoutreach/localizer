#!/usr/bin/env bash
# This script ensures we have a copy of bootstrap's libray
# framework downloaded.
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
libDir="$DIR/../.bootstrap"
lockfile="$DIR/../bootstrap.lock"

version=$(yq -r .versions.devbase <"$lockfile")
existingVersion=$(cat "$libDir/.version" 2>/dev/null || true)

if [[ ! -e $libDir ]] || [[ $existingVersion != "$version" ]] || [[ ! -e "$libDir/.version" ]]; then
  rm -rf "$libDir" || true

  git clone -q --single-branch --branch "$version" git@github.com:getoutreach/devbase "$libDir" >/dev/null
  echo -n "$version" >"$libDir/.version"
fi
