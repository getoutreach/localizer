#!/usr/bin/env bash
# Wrap protoc-gen-go for usage with protoc
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

exec "$DIR/gobin.sh" google.golang.org/protobuf/cmd/protoc-gen-go "$@"
