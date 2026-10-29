#!/bin/sh
# Retry-safe channel promotion after immutable release assets are publicly readable.
set -eu
TAG=${1:?release tag required}
shift
[ "$#" -gt 0 ] || { echo 'channel required' >&2; exit 2; }
cd "$(dirname "$0")/.."
REPO=corn12138/yan-releases
OUT=dist/$TAG
WORK=$(mktemp -d "${TMPDIR:-/tmp}/yan-promote.XXXXXX")
trap 'rm -rf "$WORK"' EXIT HUP INT TERM
fetch() { curl --disable --fail --silent --show-error --location --retry 3 --retry-all-errors --max-time 120 "$1" -o "$2"; }
BASE=https://github.com/$REPO/releases/download/$TAG
fetch "$BASE/update-darwin-arm64.json" "$WORK/update-darwin-arm64.json"
fetch "$BASE/yan-$TAG-darwin-arm64.update.tar.gz" "$WORK/yan-$TAG-darwin-arm64.update.tar.gz"
cmp "$OUT/update-darwin-arm64.json" "$WORK/update-darwin-arm64.json"
CGO_ENABLED=0 go run ./internal/tools/promoteupdates -dir "$WORK" -tag "$TAG" "$@"
