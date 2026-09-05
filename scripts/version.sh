#!/usr/bin/env bash
# Check that all materialized Eterno Mail application-version fields match VERSION.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_FILE="${VERSION_FILE:-$ROOT_DIR/VERSION}"

usage() {
    echo "Usage: $0 check [vX.Y.Z]" >&2
    exit 2
}

[[ "${1:-}" == "check" ]] || usage
[[ $# -le 2 ]] || usage
if [[ -n "${VERSION_OVERRIDE:-}" ]]; then
    VERSION="$VERSION_OVERRIDE"
else
    [[ -f "$VERSION_FILE" ]] || { echo "VERSION file not found: $VERSION_FILE" >&2; exit 2; }
    VERSION="$(tr -d '\r\n' < "$VERSION_FILE")"
fi
SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+$'
if [[ ! "$VERSION" =~ $SEMVER_RE ]]; then
    echo "Invalid VERSION '$VERSION': expected X.Y.Z" >&2
    exit 1
fi

if [[ $# -eq 2 && "$2" != "v$VERSION" ]]; then
    echo "Tag '$2' does not match VERSION '$VERSION' (expected v$VERSION)" >&2
    exit 1
fi

failures=0
check_value() {
    local label="$1" actual="$2"
    if [[ "$actual" != "$VERSION" ]]; then
        echo "$label: expected $VERSION, found ${actual:-<missing>}" >&2
        failures=1
    fi
}

json_version() {
    local file="$1" occurrence="${2:-1}"
    awk -v occurrence="$occurrence" '
        /"version"[[:space:]]*:/ {
            found++
            if (found == occurrence) {
                line=$0
                sub(/^[^:]*:[[:space:]]*"/, "", line)
                sub(/".*/, "", line)
                print line
                exit
            }
        }' "$file"
}

check_value "app/state.go" "$(awk -F'"' '/^const Version = / { print $2; exit }' "$ROOT_DIR/app/state.go")"
check_value "frontend/package.json" "$(json_version "$ROOT_DIR/frontend/package.json")"
check_value "frontend/package-lock.json root" "$(json_version "$ROOT_DIR/frontend/package-lock.json")"
check_value "frontend/package-lock.json package root" "$(json_version "$ROOT_DIR/frontend/package-lock.json" 2)"
check_value "wails.json" "$(awk -F'"' '/"productVersion"/ { print $4; exit }' "$ROOT_DIR/wails.json")"
check_value "AppStream current release" "$(awk -F'"' '/<release version=/ { print $2; exit }' "$ROOT_DIR/build/flatpak/io.github.wesleiaqui.eternomail.metainfo.xml")"
check_value "Flatpak source tag" "$(awk '/^[[:space:]]*tag: / { sub(/^[[:space:]]*tag: v/, ""); print; exit }' "$ROOT_DIR/build/flatpak/flathub/io.github.wesleiaqui.eternomail.yml")"

if (( failures )); then
    exit 1
fi

echo "Version consistency check passed: $VERSION"
