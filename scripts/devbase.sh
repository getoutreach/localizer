#!/usr/bin/env bash
# This script ensures we have a copy of devbase
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
libDir="$DIR/../.bootstrap"
lockfile="$DIR/../stencil.lock"
serviceYaml="$DIR/../service.yaml"
gojqVersion="v0.12.16"

# get_absolute_path returns the absolute path of a file
get_absolute_path() {
  python="$(command -v python3 || command -v python)"
  "$python" -c "import os,sys; print(os.path.realpath(sys.argv[1]))" "$1"
}

# gojq returns the path to a JIT-downloaded gojq binary.
gojq() {
  set -uo pipefail

  local gjDir
  gjDir="${XDG_CACHE_HOME:-$HOME/.cache}/devbase/gojq"
  local gojq="$gjDir/gojq-${gojqVersion}"
  if [[ ! -x $gojq ]]; then
    local platform arch
    mkdir -p "$gjDir"
    platform="$(uname -s | awk '{print tolower($0)}')"
    arch="$(uname -m)"
    case $arch in
    x86_64)
      arch=amd64
      ;;
    aarch64)
      arch=arm64
      ;;
    esac
    local basename="gojq_${gojqVersion}_${platform}_${arch}"
    local ext
    if [[ $platform == linux ]]; then
      ext="tar.gz"
    else
      ext="zip"
    fi
    local archive="$basename.$ext"

    local gojqURL="https://github.com/itchyny/gojq/releases/download/$gojqVersion/$archive"
    local archivePath="$gjDir/$archive"
    if [[ ! -e $archivePath ]]; then
      if command -v busybox >/dev/null; then
        busybox wget --quiet --output-document "$archivePath" "$gojqURL"
      elif command -v wget >/dev/null; then
        wget --quiet --output-document "$archivePath" "$gojqURL"
      elif command -v curl >/dev/null; then
        curl --fail --location --silent --output "$archivePath" "$gojqURL"
      else
        echo "No download tool found (looked for busybox, wget, curl)" >&2
        exit 1
      fi
    fi

    if [[ ! -e $archivePath ]]; then
      echo "Failed to download gojq ($gojqURL)" >&2
      exit 1
    fi

    if [[ $ext == "zip" ]]; then
      # Explanation of flags:
      # quiet, junk paths/dont make directories, extract to directory
      unzip -q -j -d "$gjDir" "$archivePath" "$basename/gojq"
    else
      tar --strip-components=1 --directory="$gjDir" --extract --file="$archivePath" "$basename/gojq"
    fi
    mv "$gjDir"/gojq "$gojq"
  fi

  echo "$gojq"

  set +uo pipefail
}

# get_field_from_yaml reads a field from a yaml file via a JIT-downloaded gojq.
get_field_from_yaml() {
  local field="$1"
  local file="$2"

  "$(gojq)" --yaml-input -r "$field" <"$file"
}

# Use the version of devbase from stencil
version=$(get_field_from_yaml '.modules[] | select(.name == "github.com/getoutreach/devbase") | .version' "$lockfile")
existingVersion=$(cat "$libDir/.version" 2>/dev/null || true)

if [[ ! -e $libDir ]] || [[ $existingVersion != "$version" ]] || [[ ! -e "$libDir/.version" ]]; then
  rm -rf "$libDir"

  if [[ $version == "local" ]]; then
    # If we're using a local version, we should use ln
    # to map the local devbase into the bootstrap dir

    path=$(get_field_from_yaml '.replacements["github.com/getoutreach/devbase"]' "$serviceYaml")
    absolute_path=$(get_absolute_path "$path")

    ln -sf "$absolute_path" "$libDir"
  else
    git clone -q --single-branch --branch "$version" git@github.com:getoutreach/devbase \
      "$libDir" >/dev/null
  fi

  echo -n "$version" >"$libDir/.version"
fi

# If we're not using a local version, ensure that service.yaml doesn't exist
# in the library directory. This is because of the repo detection logic
# which looks for the base directory through the existence of that file.
if [[ $version != "local" ]]; then
  if [[ -e "$libDir/service.yaml" ]]; then
    rm "$libDir/service.yaml"
  fi
fi
