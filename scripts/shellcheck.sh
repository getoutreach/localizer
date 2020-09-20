#!/usr/bin/env bash
# This is a wrapper around gobin.sh to run shellcheck.
# Useful for using the correct version of shellcheck
# with your editor.

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
BIN="$DIR/../bin"
SHELLCHECK_VERSION="0.7.1"

# Ensure $BIN exists, since GOBIN makes it, but
# we may run before it.
mkdir -p "$BIN"

GOOS=$(go env GOOS)
ARCH=$(uname -m)

tmp_dir=$(mktemp -d)
binPath="$BIN/shellcheck-$SHELLCHECK_VERSION"

# Always set the correct script directory.
args=("-P" "SCRIPTDIR" "-x" "$@")

if [[ ! -e $binPath ]]; then
  {
    echo "downloading shellcheck@$SHELLCHECK_VERSION"
    wget -O "$tmp_dir/shellcheck.tar.xz" \
      "https://github.com/koalaman/shellcheck/releases/download/v$SHELLCHECK_VERSION/shellcheck-v$SHELLCHECK_VERSION.$GOOS.$ARCH.tar.xz"
    pushd "$tmp_dir" >/dev/null || exit 1
    tar xvf shellcheck.tar.xz
    mv "shellcheck-v$SHELLCHECK_VERSION/shellcheck" "$binPath"
    chmod +x "$binPath"
    popd >/dev/null || exit 1
    rm -rf "$tmp_dir"
  } >&2
fi

exec "$binPath" "${args[@]}"
