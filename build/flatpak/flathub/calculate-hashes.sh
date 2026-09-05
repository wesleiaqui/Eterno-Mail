#!/usr/bin/env bash
# Update the source-only Flathub manifest for a release tag.

set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <version-or-tag>" >&2
    exit 1
fi

TAG_NAME="$1"
REPO="https://github.com/wesleiaqui/EternoMail"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="$SCRIPT_DIR/io.github.wesleiaqui.eternomail.yml"

if [ ! -f "$MANIFEST" ]; then
    echo "ERROR: Manifest file not found: $MANIFEST" >&2
    exit 1
fi

# Annotated tags expose their target through the peeled ^{} ref; lightweight tags
# only have the direct ref. Prefer the peeled commit when it is available.
COMMIT_HASH="$(git ls-remote "$REPO" "refs/tags/$TAG_NAME" "refs/tags/$TAG_NAME^{}" |
    awk -v tag="refs/tags/$TAG_NAME" '
        $2 == tag "^{}" { peeled = $1 }
        $2 == tag { direct = $1 }
        END { if (peeled != "") print peeled; else if (direct != "") print direct }
    ')"

if [ -z "$COMMIT_HASH" ]; then
    echo "ERROR: Could not resolve commit hash for tag $TAG_NAME" >&2
    exit 1
fi

TMP_MANIFEST="$(mktemp "${MANIFEST}.XXXXXX")"
trap 'rm -f "$TMP_MANIFEST"' EXIT

awk -v tag="$TAG_NAME" -v commit="$COMMIT_HASH" '
    /^[[:space:]]*tag:[[:space:]]/ {
        sub(/tag:.*/, "tag: " tag)
    }
    /^[[:space:]]*commit:[[:space:]]/ {
        sub(/commit:.*/, "commit: " commit)
    }
    { print }
' "$MANIFEST" > "$TMP_MANIFEST"
cat "$TMP_MANIFEST" > "$MANIFEST"
rm -f "$TMP_MANIFEST"
trap - EXIT

echo "Updated manifest for $TAG_NAME ($COMMIT_HASH)"
