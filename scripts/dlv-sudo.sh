#!/usr/bin/env bash
exec sudo -E "$(command -v dlv)" --only-same-user=false "$@"
