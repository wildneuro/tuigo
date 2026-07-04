#!/bin/bash
# Bumps VERSION, runs the CI checks, regenerates demo.gif, and commits the
# result locally — no push, no GitHub release, no cross-platform binaries.
# Adapted from the eee project's release.sh, stripped down per this repo's
# needs: tuigo isn't shipping distributable binaries yet, just committing a
# versioned snapshot (with a fresh demo) to this repo.
#
# Usage: ./release.sh [patch|minor|major|vX.Y.Z]   (default: patch)
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

VERSION_FILE="VERSION"
[ -f "$VERSION_FILE" ] || echo "v0.1.0" > "$VERSION_FILE"

CURRENT=$(tr -d ' v' < "$VERSION_FILE")
MAJOR=$(echo "$CURRENT" | cut -d. -f1)
MINOR=$(echo "$CURRENT" | cut -d. -f2)
PATCH=$(echo "$CURRENT" | cut -d. -f3)

BUMP="${1:-patch}"
case "$BUMP" in
    major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    patch) PATCH=$((PATCH + 1)) ;;
    v*)
        VERSION="$BUMP"
        ;;
    *)
        echo "Usage: $0 [patch|minor|major|vX.Y.Z]"
        echo "  Default: patch bump  (current: v$CURRENT)"
        exit 1
        ;;
esac
: "${VERSION:=v$MAJOR.$MINOR.$PATCH}"

echo "=== Releasing $VERSION (was v$CURRENT) ==="

echo ""
echo "=== Running checks ==="
go build ./...
go vet ./...
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "gofmt found unformatted files:"
    echo "$UNFORMATTED"
    exit 1
fi
go test ./...

echo ""
echo "=== Generating demo GIF ==="
./make-demo.sh || echo "warning: demo GIF generation failed, continuing without it"

echo "$VERSION" > "$VERSION_FILE"

echo ""
echo "=== Committing (staged files + VERSION + demo.gif; no push) ==="
git add "$VERSION_FILE"
if [ -f demo.gif ]; then
    git add demo.gif
fi
git commit -m "release $VERSION"

echo ""
echo "=== Done: $VERSION committed locally ==="
git log -1 --stat
