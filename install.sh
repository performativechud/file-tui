#!/usr/bin/env bash
# file-tui installer — downloads the latest release binary for your platform
# Usage: curl -fsSL https://raw.githubusercontent.com/performativechud/file-tui/main/install.sh | bash
# Or:    curl -fsSL https://raw.githubusercontent.com/performativechud/file-tui/main/install.sh | bash -s -- -d ~/.local/bin

set -euo pipefail

REPO="performativechud/file-tui"
BINARY="file-tui"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Parse flags
while [[ $# -gt 0 ]]; do
  case $1 in
    -d|--dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [-d|--dir DIR]"
      echo "  -d, --dir DIR  Install directory (default: /usr/local/bin, or ~/.local/bin if not root)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  GOOS="linux" ;;
  darwin) GOOS="darwin" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64)  GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# If not root and default install dir, use ~/.local/bin
if [[ "$INSTALL_DIR" == "/usr/local/bin" && $EUID -ne 0 ]]; then
  INSTALL_DIR="$HOME/.local/bin"
fi

# Create install directory
mkdir -p "$INSTALL_DIR"

# Get latest release version
echo "Fetching latest release..."
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
if [[ -z "$VERSION" ]]; then
  echo "Failed to fetch latest version. Are releases published?"
  exit 1
fi
echo "Latest version: $VERSION"

# Download URL
URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY}-${GOOS}-${GOARCH}.tar.gz"
echo "Downloading from: $URL"

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -fsSL "$URL" | tar -xz -C "$TMPDIR"; then
  echo "Download failed. Binary may not exist for $GOOS/$GOARCH."
  echo "Try building from source: go install github.com/performativechud/file-tui@latest"
  exit 1
fi

# Install binary
install -m 755 "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
echo "Installed $BINARY to $INSTALL_DIR/$BINARY"

# Verify
if command -v "$BINARY" >/dev/null 2>&1; then
  echo "Success! Run '$BINARY --version' to verify."
else
  echo "Note: $INSTALL_DIR may not be in your PATH. Add it to your shell config:"
  echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
fi