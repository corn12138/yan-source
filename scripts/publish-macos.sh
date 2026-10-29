#!/bin/sh
# Publish binary-only releases to the public distribution repository. Promote the channel last.
set -eu
TAG=${1:-}
case "$TAG" in v[0-9]*) ;; *) echo "usage: $0 v<version>" >&2; exit 2 ;; esac
cd "$(dirname "$0")/.."
REPO=corn12138/yan-releases
PUBLIC_BRANCH=$(gh repo view "$REPO" --json defaultBranchRef --jq .defaultBranchRef.name)
OUT=dist/$TAG
NOTES=docs/releases/$TAG.md
fail() { echo "publish: $*" >&2; exit 1; }
[ -f "$NOTES" ] || fail "missing $NOTES"
for f in "yan-$TAG-macos-arm64.dmg" "yan-$TAG-macos-arm64.app.zip" "yan-$TAG-darwin-arm64.update.tar.gz" update-darwin-arm64.json THIRD_PARTY_LICENSES.txt SHA256SUMS; do
 [ -f "$OUT/$f" ] || fail "missing $OUT/$f"
done
(cd "$OUT" && shasum -a 256 -c SHA256SUMS >/dev/null) || fail "checksums differ"
CGO_ENABLED=0 go run ./internal/tools/updatepack -check -tag "$TAG" -out "$OUT"
[ "$(git describe --exact-match --tags HEAD)" = "$TAG" ] || fail "source checkout is not the release tag"
[ -z "$(git status --porcelain)" ] || fail "source checkout has changes"
[ -n "$(git ls-remote --tags origin "refs/tags/$TAG")" ] || fail "source tag is not pushed"
[ "$(gh repo view "$REPO" --json visibility --jq .visibility)" = PUBLIC ] || fail "distribution repository is not public"
! gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1 || fail "$TAG already exists; do not replace a published version"
TITLE="yan $TAG — macOS (Apple Silicon)"
PRE=
CHANNELS='stable preview'
case "$TAG" in *-alpha.*) PRE=--prerelease; CHANNELS=preview; TITLE="yan $TAG — macOS preview (Apple Silicon)" ;; esac
printf 'Publish %s to %s? [y/N] ' "$TAG" "$REPO"
read -r answer || answer=
case "$answer" in [yY]*) ;; *) exit 0 ;; esac
# Distribution tags identify the release catalog independently of source tags.
# shellcheck disable=SC2086
 gh release create "$TAG" --repo "$REPO" --target "$PUBLIC_BRANCH" --draft $PRE --title "$TITLE" --notes-file "$NOTES" \
 "$OUT/yan-$TAG-macos-arm64.dmg" "$OUT/yan-$TAG-macos-arm64.app.zip" \
 "$OUT/yan-$TAG-darwin-arm64.update.tar.gz" "$OUT/update-darwin-arm64.json" \
 "$OUT/THIRD_PARTY_LICENSES.txt" "$OUT/SHA256SUMS"
gh release edit "$TAG" --repo "$REPO" --draft=false --latest=false
scripts/promote-updates.sh "$TAG" $CHANNELS
