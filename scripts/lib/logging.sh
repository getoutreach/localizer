#!/usr/bin/env bash

info() {
  echo -e " \033[32m::\033[0m $*"
}

error() {
  echo -e "\033[31mError:\033[0m $*" >&2
}
