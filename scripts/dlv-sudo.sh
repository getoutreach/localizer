#!/usr/bin/env bash
exec sudo -E dlv --only-same-user=false "$@"
