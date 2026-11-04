#!/bin/sh
set -e

# Generate website/apps/web/public/version.json with the latest release info.
# Called by goreleaser before build. The version is derived from the git tag.

VERSION="$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
# GoReleaser sets GITHUB_REF_NAME on tag builds; use it when git tags are unavailable.
if [ -z "$VERSION" ] && [ -n "${GITHUB_REF_NAME:-}" ]; then
  VERSION="${GITHUB_REF_NAME#v}"
fi
if [ -z "$VERSION" ]; then
  echo "cannot determine release version: no git tag and GITHUB_REF_NAME is empty" >&2
  exit 1
fi

TAG="v${VERSION}"
DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
REPO="PxeLab/pxelab"

# website/ is a submodule and may be absent (e.g. release CI checkouts)
mkdir -p website/apps/web/public

cat > website/apps/web/public/version.json <<- VERSION_EOF
{
  "latest_version": "${VERSION}",
  "release_date": "${DATE}",
  "release_notes_url": "https://github.com/${REPO}/releases/tag/${TAG}",
  "download_url": "https://github.com/${REPO}/releases/tag/${TAG}",
  "checksums_url": "https://github.com/${REPO}/releases/download/${TAG}/pxelab_${VERSION}_checksums.txt",
  "min_upgrade_version": ""
}
VERSION_EOF
