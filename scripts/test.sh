#!/usr/bin/env bash

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
LINTER="$DIR/golangci-lint.sh"
SHELLFMTPATH="$DIR/shfmt.sh"
SHELLCHECKPATH="$DIR/shellcheck.sh"

# shellcheck source=./lib/logging.sh
source "$DIR/lib/logging.sh"

if [[ -n $CI ]]; then
  TEST_TAGS=${TEST_TAGS:-tm_test,tm_int}
else
  TEST_TAGS=${TEST_TAGS:-tm_test}
fi

if [[ $TEST_TAGS == *"tm_int"* ]]; then
  BENCH_FLAGS=${BENCH_FLAGS:--bench=^Bench -benchtime=1x}
fi

# Run shellcheck on shell-scripts, only if installed.
info "Running shellcheck"
if ! git ls-files '*.sh' | xargs -n1 "$SHELLCHECKPATH"; then
  error "shellcheck failed on some files"
  exit 1
fi

info "Running shfmt"
if ! git ls-files '*.sh' | xargs -n1 "$SHELLFMTPATH" -d; then
  error "shfmt failed on some files"
  exit 1
fi

# TODO(jaredallard): enable golangci-lint
info "Running $(basename "$LINTER" .sh)"
"$LINTER" run --build-tags "$TEST_TAGS" ./...

info "Running go test ($TEST_TAGS)"
# Why: We want these to split.
# shellcheck disable=SC2086
go test $BENCH_FLAGS \
  -ldflags "-X github.com/tritonmedia/pkg/app.Version=testing" -tags="$TEST_TAGS" \
  -covermode=atomic -coverprofile=/tmp/coverage.out -cover "$@" ./...
