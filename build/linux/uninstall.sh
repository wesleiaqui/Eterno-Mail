#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

echo -e "${PURPLE}
 ░▒▓██████▓▒░░▒▓████████▓▒░▒▓███████▓▒░░▒▓█▓▒░░▒▓██████▓▒░░▒▓███████▓▒░
░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
░▒▓████████▓▒░▒▓██████▓▒░ ░▒▓███████▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░      ░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓█▓▒░
░▒▓█▓▒░░▒▓█▓▒░▒▓████████▓▒░▒▓█▓▒░░▒▓█▓▒░▒▓█▓▒░░▒▓██████▓▒░░▒▓█▓▒░░▒▓█▓▒░

${NC}
"

# Print colored output
print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_info() {
    echo -e "${YELLOW}$1${NC}"
}

# Check if running on Linux
if [[ "$(uname -s)" != "Linux" ]]; then
    print_error "This script is only for Linux systems"
    exit 1
fi

echo ""
print_info "Eterno Mail - Uninstall Script"
echo ""
echo "This script will uninstall Eterno Mail from your system."
echo ""
echo "Choose installation type to uninstall:"
echo "  1) System-wide (requires sudo, /usr/local or /usr/share)"
echo "  2) User only (~/.local)"
echo ""

while true; do
    read -p "Enter your choice (1 or 2): " choice
    case $choice in
        1)
            INSTALL_TYPE="system"
            BIN_DIR="/usr/local/bin"
            APPS_DIR="/usr/share/applications"
            ICONS_DIR="/usr/share/icons/hicolor/256x256/apps"
            NEEDS_SUDO=true
            break
            ;;
        2)
            INSTALL_TYPE="user"
            BIN_DIR="$HOME/.local/bin"
            APPS_DIR="$HOME/.local/share/applications"
            ICONS_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
            NEEDS_SUDO=false
            break
            ;;
        *)
            print_error "Invalid choice. Please enter 1 or 2."
            ;;
    esac
done

echo ""
print_info "Uninstalling Eterno Mail ($INSTALL_TYPE)..."
echo ""

# Function to run command with or without sudo
run_cmd() {
    if [[ "$NEEDS_SUDO" == true ]]; then
        sudo "$@"
    else
        "$@"
    fi
}

# Function to ask for confirmation and remove file
remove_file() {
    local file="$1"
    local description="$2"

    if [[ -f "$file" ]]; then
        echo ""
        read -p "Remove $description at $file? (y/N): " confirm
        case $confirm in
            [yY]|[yY][eE][sS])
                run_cmd rm -f "$file"
                print_success "✓ Removed $description"
                return 0
                ;;
            *)
                print_info "Skipped $description"
                return 1
                ;;
        esac
    else
        print_info "$description not found at $file (already removed or not installed)"
        return 1
    fi
}

# Track if anything was removed
REMOVED_COUNT=0

# Remove binary
if remove_file "$BIN_DIR/eterno-mail" "binary"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove new desktop file
if remove_file "$APPS_DIR/io.github.wesleiaqui.eternomail.desktop" "desktop file"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove old desktop file if it exists
if remove_file "$APPS_DIR/aerion.desktop" "old desktop file"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove legacy Eterno Mail desktop file if it exists
if remove_file "$APPS_DIR/io.github.hkdb.EternoMail.desktop" "legacy desktop file"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove backup desktop file if it exists
if remove_file "$APPS_DIR/aerion.desktop.backup" "backup desktop file"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove legacy Eterno Mail backup desktop file if it exists
if remove_file "$APPS_DIR/io.github.hkdb.EternoMail.desktop.backup" "legacy backup desktop file"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove icon
if remove_file "$ICONS_DIR/io.github.wesleiaqui.eternomail.png" "icon"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove old icon name if it exists
if remove_file "$ICONS_DIR/aerion.png" "old icon"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

# Remove legacy Eterno Mail icon name if it exists
if remove_file "$ICONS_DIR/io.github.hkdb.EternoMail.png" "legacy icon"; then
    REMOVED_COUNT=$((REMOVED_COUNT + 1))
fi

echo ""

# Update caches if anything was removed
if [[ $REMOVED_COUNT -gt 0 ]]; then
    print_info "Updating system caches..."

    # Update icon cache
    if [[ "$INSTALL_TYPE" == "system" ]]; then
        run_cmd gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
    else
        gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
    fi

    # Update desktop database
    if [[ "$INSTALL_TYPE" == "system" ]]; then
        run_cmd update-desktop-database /usr/share/applications 2>/dev/null || true
    else
        update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true
    fi

    echo ""
    print_success "✓ Uninstall complete! ($REMOVED_COUNT file(s) removed)"
else
    print_info "No files were removed."
fi

echo ""
echo "Thank you for trying Eterno Mail!"
echo ""
