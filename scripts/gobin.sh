#!/usr/bin/env bash
#
# Run a golang binary using gobin

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
GOBINVERSION=v0.0.14
GOBINPATH="$DIR/../bin/gobin"
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

PRINT_PATH=false
if [[ $1 == "-p" ]]; then
  PRINT_PATH=true
  shift
fi

if [[ -z $1 ]] || [[ $1 =~ ^(--help|-h) ]]; then
  echo "Usage: $0 [-p|-h|--help] <package> [args]" >&2
  exit 1
fi

if [[ ! -e $GOBINPATH ]]; then
  mkdir -p "$(dirname "$GOBINPATH")"
  echo "installing gobin into '$GOBINPATH'" >&2
  curl -L -o "$GOBINPATH" "https://github.com/myitcv/gobin/releases/download/$GOBINVERSION/$GOOS-$GOARCH" >&2
  chmod +x "$GOBINPATH"
fi

package="$1"
shift

# Look up versions inside of .tool-versions
if ! grep "@" <<<"$package" >/dev/null 2>&1; then
  if [[ -e "$DIR/../.tool-versions" ]]; then
    version=$(grep "$package" "$DIR/../.tool-versions" | awk '{ print $2 }')
    if [[ -n $version ]]; then
      package="$package@$version"
    fi
  fi
fi

if [[ $PRINT_PATH == "true" ]]; then
  exec "$GOBINPATH" -p "$package"
fi

exec "$GOBINPATH" -run "$package" "$@"
