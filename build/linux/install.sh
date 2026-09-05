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

# Check if binary exists
if [[ ! -f "eterno-mail" ]]; then
    print_error "eterno-mail binary not found in current directory"
    echo "Please run this script from the directory containing the eterno-mail binary"
    exit 1
fi

# Check if desktop file exists
if [[ ! -f "io.github.wesleiaqui.EternoMail.desktop" ]]; then
    print_error "io.github.wesleiaqui.EternoMail.desktop file not found in current directory"
    echo "Please ensure the desktop file is in the same directory as this script"
    exit 1
fi

# Check if icon exists
if [[ ! -f "io.github.wesleiaqui.EternoMail.png" ]]; then
    print_error "io.github.wesleiaqui.EternoMail.png icon not found in current directory"
    echo "Please ensure the icon file is in the same directory as this script"
    exit 1
fi

echo ""
print_info "Eterno Mail - Installation Script"
echo ""
echo "This script will install Eterno Mail on your system."
echo ""
echo "Choose installation type:"
echo "  1) System-wide (requires sudo, installs to /usr/local)"
echo "  2) User only (installs to ~/.local)"
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
print_info "Installing Eterno Mail ($INSTALL_TYPE)..."
echo ""

# Function to run command with or without sudo
run_cmd() {
    if [[ "$NEEDS_SUDO" == true ]]; then
        sudo "$@"
    else
        "$@"
    fi
}

# Move legacy desktop entries out of the launcher search path.
for legacy_desktop in aerion.desktop io.github.hkdb.EternoMail.desktop; do
    if [[ -f "$APPS_DIR/$legacy_desktop" ]]; then
        print_info "Found legacy $legacy_desktop, renaming it to a backup..."
        run_cmd mv "$APPS_DIR/$legacy_desktop" "$APPS_DIR/$legacy_desktop.backup"
        print_success "Legacy desktop file renamed to $legacy_desktop.backup"
    fi
done

# Create directories if they don't exist
print_info "Creating directories..."
run_cmd mkdir -p "$BIN_DIR"
run_cmd mkdir -p "$APPS_DIR"
run_cmd mkdir -p "$ICONS_DIR"

# Remove icons that belong to legacy desktop identifiers.
for legacy_icon in aerion.png io.github.hkdb.EternoMail.png; do
    if [[ -f "$ICONS_DIR/$legacy_icon" ]]; then
        run_cmd rm -f "$ICONS_DIR/$legacy_icon"
    fi
done

# Install binary
print_info "Installing binary to $BIN_DIR..."
run_cmd install -Dm755 eterno-mail "$BIN_DIR/eterno-mail"

# Install desktop file
print_info "Installing desktop file to $APPS_DIR..."
run_cmd install -Dm644 io.github.wesleiaqui.EternoMail.desktop "$APPS_DIR/io.github.wesleiaqui.EternoMail.desktop"

# Install icon
print_info "Installing icon to $ICONS_DIR..."
run_cmd install -Dm644 io.github.wesleiaqui.EternoMail.png "$ICONS_DIR/io.github.wesleiaqui.EternoMail.png"

# Update icon cache
print_info "Updating icon cache..."
if [[ "$INSTALL_TYPE" == "system" ]]; then
    run_cmd gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
else
    gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" 2>/dev/null || true
fi

# Update desktop database
print_info "Updating desktop database..."
if [[ "$INSTALL_TYPE" == "system" ]]; then
    run_cmd update-desktop-database /usr/share/applications 2>/dev/null || true
else
    update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true
fi

echo ""
print_success "✓ Installation complete!"
echo ""

# Additional setup instructions
if [[ "$INSTALL_TYPE" == "user" ]]; then
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        print_info "Note: $HOME/.local/bin is not in your PATH"
        echo "You may need to add it to your PATH by adding this line to your ~/.bashrc or ~/.zshrc:"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo ""
    fi
fi

echo "You may need to log out and back in for the application to appear in your menu."
echo ""
echo "To set Eterno Mail as your default email client, run:"
echo "  xdg-mime default io.github.wesleiaqui.EternoMail.desktop x-scheme-handler/mailto"
echo ""
echo "To start Eterno Mail, run:"
echo "  eterno-mail --dbus-notify"
echo ""
