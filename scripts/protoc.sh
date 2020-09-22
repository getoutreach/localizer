#!/usr/bin/env bash
# Run protoc using the correct versions of protoc-gen-go[-grpc].
# This will eventually ensure the correct version of protoc is being used.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

set -x

protoc -I. --plugin=protoc-gen-go="$DIR/protoc-gen-go.sh" \
  --plugin=protoc-gen-go_grpc="$DIR/protoc-gen-go-grpc.sh" \
  --go_grpc_out=paths=source_relative:. --go_out=paths=source_relative:. "$@"
