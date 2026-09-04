# BUILD

Set .env file

### 🔨 Building from Source
---

**Flatpak (Recommended for Linux):**

```bash
# Install flatpak-builder
sudo dnf install flatpak-builder  # Fedora
sudo apt install flatpak-builder  # Ubuntu/Debian
sudo pacman -S flatpak-builder    # Arch

# Set Microsoft and Google oAuth creds
cp .env.example .env
# Fill in your own creds

# Or via make
make flatpak

# Install
flatpak --user install build/bin/Eterno Mail.flatpak

# Run
flatpak run io.github.hkdb.Eterno Mail
```

See [build/flatpak/README.md](../build/flatpak/README.md) for detailed Flatpak build instructions and Flathub submission guide.

**Native Binary:**

```bash
# Install dependencies (Ubuntu/Debian)
sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev

# Set Microsoft and Google oAuth creds
cp .env.example .env
# Fill in your own creds

# Build
make build

# Run
./build/bin/aerion
```


