#!/usr/bin/env bash
# Builds a Docker Container based on what branch a user is on
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# shellcheck source=../scripts/lib/logging.sh
source "$DIR/../scripts/lib/logging.sh"

if [[ -z $CI ]]; then
  error "not running in CI"
  exit 1
fi

COMMIT_BRANCH="$CIRCLE_BRANCH"

# Support Github Actions
if [[ -n $GITHUB_WORKFLOW ]]; then
  COMMIT_BRANCH="${GITHUB_HEAD_REF//refs\/heads\//}"
fi

appName="$(basename "$(pwd)")"
VERSION="$(make version)"
remote_image_name="docker.io/jaredallard/$appName"

if [[ -z $IMAGE_PUSH_SECRET ]]; then
  # Why: We're literally having it be escaped.
  # shellcheck disable=SC2016
  error 'Missing $IMAGE_PUSH_SECRET'
  exit 1
fi

info "setting up docker authn"
docker login \
  -u jaredallard \
  --password-stdin <<<"${IMAGE_PUSH_SECRET}"

info "building docker image"
docker buildx build --platform "linux/amd64,linux/arm64" \
  --cache-to "type=local,dest=/tmp/.buildx-cache" \
  --cache-from "type=local,src=/tmp/.buildx-cache" \
  -t "$appName" \
  --file "Dockerfile" \
  --build-arg "VERSION=${VERSION}" \
  .

# tag images as a PR if they are a PR
TAGS=()
if [[ $COMMIT_BRANCH == "master" ]]; then
  TAGS+=("$VERSION" "latest")
else
  # strip the branch name of invalid spec characters
  TAGS+=("$VERSION-branch.${COMMIT_BRANCH//[^a-zA-Z\-\.]/-}")

  # TODO(jaredallard): Better support multiple images at somepoint?
  echo "::set-env name=PREVIEW_IMAGE::$remote_image_name:$(cut -c 1-127 <<<"${TAGS[0]}")"
fi

for tag in "${TAGS[@]}"; do
  # fqin is the fully-qualified image name, it's tag is truncated to 127 characters to match the
  # docker tag length spec: https://docs.docker.com/engine/reference/commandline/tag/
  fqin="$remote_image_name:$(cut -c 1-127 <<<"$tag")"

  info "pushing image '$fqin'"
  docker buildx build --platform "linux/amd64,linux/arm64" \
    --cache-from "type=local,src=/tmp/.buildx-cache" \
    -t "$fqin" \
    --file "Dockerfile" \
    --push \
    --build-arg "VERSION=${VERSION}" \
    . >/dev/null
done
