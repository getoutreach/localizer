#!/usr/bin/env bash
# This script ensures we have a copy of devbase
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
libDir="$DIR/../.bootstrap"
lockfile="$DIR/../bootstrap.lock"

# get_field_from_yaml reads a field from a yaml file using either go-yq or python-yq
get_field_from_yaml() {
  field="$1"
  file="$2"

  if [[ "$(yq e '.a' '-' <<<'{"a": "true"}' 2>&1)" == "true" ]]; then
    # using golang version
    yq e "$field" "$file"
  else
    # probably using python version
    yq -r "$field" <"$file"
  fi
}

version=$(get_field_from_yaml .versions.devbase "$lockfile")
existingVersion=$(cat "$libDir/.version" 2>/dev/null || true)

if [[ ! -e $libDir ]] || [[ $existingVersion != "$version" ]] || [[ ! -e "$libDir/.version" ]]; then
  rm -rf "$libDir" || true

  git clone -q --single-branch --branch "$version" git@github.com:getoutreach/devbase "$libDir" >/dev/null
  echo -n "$version" >"$libDir/.version"

  # Don't let devbase be confused by the existence of one there :(
  rm "$libDir/service.yaml"
fi
