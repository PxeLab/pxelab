#!/bin/sh
set -e

# Generate website/apps/web/public/version.json with the latest release info.
# Called by goreleaser before build. The version is derived from the git tag.

VERSION="$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")"
DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

cat > website/apps/web/public/version.json <<- VERSION_EOF
{
  "latest_version": "${VERSION}",
  "release_date": "${DATE}",
  "release_notes_url": "https://github.com/pxelab/pxelab/releases/tag/${VERSION}",
  "download_url": "https://github.com/pxelab/pxelab/releases/download/${VERSION}",
  "checksums_url": "https://github.com/pxelab/pxelab/releases/download/${VERSION}/pxelab_${VERSION}_checksums.txt",
  "min_upgrade_version": ""
}
VERSION_EOF
