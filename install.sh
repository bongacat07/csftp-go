#!/usr/bin/env bash
set -e

VERSION="v0.1.0"
echo "Installing CSFTP version $VERSION..."

OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [[ "$ARCH" == "arm64" || "$ARCH" == "aarch64" ]]; then ARCH="arm64"; fi

# Determine file
if [[ "$OS" == "linux" || "$OS" == "darwin" ]]; then
    FILE="csftp-$OS-$ARCH"
elif [[ "$OS" == "msys"* || "$OS" == "cygwin"* || "$OS" == "windows" ]]; then
    OS="windows"
    FILE="csftp-$OS-$ARCH.exe"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

URL="https://github.com/bongacat07/csftp-go/releases/download/$VERSION/$FILE"

# Download
echo "Downloading $FILE..."
curl -L -o csftp "$URL"

# Make executable & move to ~/.local/bin
if [[ "$OS" != "windows" ]]; then
    chmod +x csftp
    mkdir -p ~/.local/bin
    mv csftp ~/.local/bin/
    echo "Installed to ~/.local/bin. Make sure it's in your PATH."
else
    INSTALL_DIR="$USERPROFILE\\bin"
    mkdir -p "$INSTALL_DIR"
    mv csftp "$INSTALL_DIR\\csftp.exe"
    echo "Installed to $INSTALL_DIR. Add it to your PATH."
fi

echo "CSFTP installation complete!"
