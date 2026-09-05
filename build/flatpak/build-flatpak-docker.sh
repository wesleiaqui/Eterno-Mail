#!/bin/bash
# Build Eterno Mail Flatpak using Docker (hybrid approach: build binary on host in container, then package)

set -e

cd "$(dirname "$0")/../.."

echo "=== Eterno Mail Flatpak Docker Builder ==="
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed"
    echo ""
    echo "Install it with:"
    echo "  Ubuntu/Debian: sudo apt install docker.io"
    echo "  Fedora:        sudo dnf install docker"
    echo "  Arch:          sudo pacman -S docker"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    echo "❌ Docker daemon is not running"
    echo ""
    echo "Start it with:"
    echo "  sudo systemctl start docker"
    exit 1
fi

echo "Building Docker image (this may take a few minutes on first run)..."
docker build -t aerion-flatpak-builder -f build/flatpak/Dockerfile build/flatpak

echo ""
echo "Building Eterno Mail in Docker container..."
echo ""

# Get version from git tag
VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "dev")

# Run the build in Docker
docker run --rm \
    -v "$(pwd):/workspace" \
    -w /workspace \
    aerion-flatpak-builder \
    bash -c "
        echo 'Installing frontend dependencies...'
        cd frontend && npm ci && cd ..

        echo ''
        echo 'Building Eterno Mail binary...'
        make build-linux

        echo ''
        echo 'Packaging into Flatpak...'
        flatpak-builder --force-clean --repo=repo build-dir build/flatpak/flathub/io.github.wesleiaqui.eternomail.yml

        echo ''
        echo 'Creating .flatpak bundle...'
        mkdir -p build/bin
        flatpak build-bundle repo build/bin/Eterno-Mail-${VERSION}.flatpak io.github.wesleiaqui.eternomail
    "

echo ""
echo "✅ Build complete!"
echo ""
echo "Flatpak bundle created: build/bin/Eterno-Mail-${VERSION}.flatpak"
echo ""
echo "To install locally:"
echo "  flatpak install --user build/bin/Eterno-Mail-${VERSION}.flatpak"
echo ""
echo "To run:"
echo "  flatpak run io.github.wesleiaqui.eternomail"
