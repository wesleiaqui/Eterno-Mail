FROM ubuntu:noble-20260113

ENV DEBIAN_FRONTEND=noninteractive

# Install build dependencies
RUN apt-get update && apt-get install -y \
    flatpak \
    flatpak-builder \
    git \
    libgtk-3-dev \
    libwebkit2gtk-4.1-dev \
    build-essential \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.23
RUN wget -q https://go.dev/dl/go1.23.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz && \
    rm go1.23.0.linux-amd64.tar.gz

# Install Node.js 20
RUN wget -qO- https://nodejs.org/dist/v20.11.0/node-v20.11.0-linux-x64.tar.xz | tar -xJ -C /usr/local && \
    ln -s /usr/local/node-v20.11.0-linux-x64/bin/node /usr/local/bin/node && \
    ln -s /usr/local/node-v20.11.0-linux-x64/bin/npm /usr/local/bin/npm

# Add Go and Node to PATH
ENV PATH="/usr/local/go/bin:/usr/local/node-v20.11.0-linux-x64/bin:${PATH}"

# Install Wails CLI
RUN go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
ENV PATH="/root/go/bin:${PATH}"

# Add Flathub repository
RUN flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo

# Install Flatpak runtimes (only Platform and SDK, no extensions needed)
RUN flatpak install -y flathub \
    org.gnome.Platform//47 \
    org.gnome.Sdk//47

WORKDIR /workspace
