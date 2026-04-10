#!/bin/sh
# glasp installer for Linux and macOS
# Usage: curl -sSL https://takihito.github.io/glasp/install.sh | sh
set -eu

REPO="takihito/glasp"
INSTALL_DIR="${GLASP_INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  *)       echo "Error: unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)             echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
echo "Fetching latest version..."
VERSION="$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
if [ -z "$VERSION" ]; then
  echo "Error: failed to fetch latest version"
  exit 1
fi
echo "Latest version: $VERSION"

# Download
ARTIFACT="glasp_${VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${ARTIFACT}..."
curl -sSL -o "${TMPDIR}/${ARTIFACT}" "${BASE_URL}/${ARTIFACT}"
curl -sSL -o "${TMPDIR}/${CHECKSUMS}" "${BASE_URL}/${CHECKSUMS}"

# Verify checksum
echo "Verifying checksum..."
cd "$TMPDIR"
if command -v sha256sum >/dev/null 2>&1; then
  grep "  ${ARTIFACT}$" "${CHECKSUMS}" | sha256sum -c --quiet
elif command -v shasum >/dev/null 2>&1; then
  grep "  ${ARTIFACT}$" "${CHECKSUMS}" | shasum -a 256 -c --quiet
else
  echo "Warning: no checksum tool found, skipping verification"
fi

# Extract and install
echo "Installing to ${INSTALL_DIR}/glasp..."
tar -xzf "${ARTIFACT}" glasp
if [ -w "$INSTALL_DIR" ]; then
  mv glasp "${INSTALL_DIR}/glasp"
else
  sudo mv glasp "${INSTALL_DIR}/glasp"
fi

echo ""
echo "glasp ${VERSION} installed successfully!"
echo "Run 'glasp version' to verify."
