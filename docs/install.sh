#!/bin/sh
# glasp installer for Linux and macOS
# Usage: curl -fsSL https://takihito.github.io/glasp/install.sh | sh
#
# Environment variables:
#   GLASP_INSTALL_DIR       Install directory (default: ~/.local/bin)
#   GLASP_VERSION           Version to install (default: latest release)
#   GLASP_VERIFY_SIGNATURE  "auto" (default): verify checksums.txt with cosign
#                           when cosign is installed, warn otherwise.
#                           "required": fail when cosign is not installed.
set -eu

REPO="takihito/glasp"
VERIFY_SIGNATURE="${GLASP_VERIFY_SIGNATURE:-auto}"
case "$VERIFY_SIGNATURE" in
  auto|required) ;;
  *) echo "Error: GLASP_VERIFY_SIGNATURE must be \"auto\" or \"required\": $VERIFY_SIGNATURE"; exit 1 ;;
esac

# HTTPS only (also across redirects), TLS 1.2+, and fail on HTTP errors.
fetch() {
  curl -fsSL --proto '=https' --tlsv1.2 "$@"
}
INSTALL_DIR="${GLASP_INSTALL_DIR:-${HOME}/.local/bin}"

# Resolve INSTALL_DIR to absolute path
case "$INSTALL_DIR" in
  /*) ;;
  ~/*) INSTALL_DIR="${HOME}/${INSTALL_DIR#~/}" ;;
  *) INSTALL_DIR="$(pwd)/${INSTALL_DIR}" ;;
esac

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

# Detect Rosetta 2 on macOS
if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then
  if [ "$(sysctl -n sysctl.proc_translated 2>/dev/null)" = "1" ]; then
    ARCH="arm64"
  fi
fi

# Select checksum tool
SHASUM=""
if command -v shasum >/dev/null 2>&1; then
  SHASUM="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
  SHASUM="sha256sum"
else
  echo "Error: neither shasum nor sha256sum found. Install one and retry."
  exit 1
fi

# Resolve version: positional arg > GLASP_VERSION env var > latest release
VERSION="${1:-${GLASP_VERSION:-}}"
if [ -z "$VERSION" ]; then
  echo "Fetching latest version..."
  VERSION="$(fetch "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
  if [ -z "$VERSION" ]; then
    echo "Error: failed to fetch latest version"
    exit 1
  fi
fi
echo "Version: $VERSION"

# Download to temp directory
ARTIFACT="glasp_${VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
WORK="$(mktemp -d 2>/dev/null || mktemp -d -t glasp)"
trap 'rm -rf "$WORK"' EXIT

echo "Downloading ${ARTIFACT}..."
fetch -o "${WORK}/${ARTIFACT}" "${BASE_URL}/${ARTIFACT}"
fetch -o "${WORK}/${CHECKSUMS}" "${BASE_URL}/${CHECKSUMS}"

# Verify the checksums.txt signature. The signature is checked against the
# Sigstore trust root, independently of the TLS certificate of the download,
# so a tampered checksums.txt is rejected even if the connection was
# intercepted. Only the release workflow of this repository, at this tag, is
# accepted as the signer.
if command -v cosign >/dev/null 2>&1; then
  echo "Verifying signature of ${CHECKSUMS}..."
  BUNDLE="${CHECKSUMS}.sigstore.json"
  if ! fetch -o "${WORK}/${BUNDLE}" "${BASE_URL}/${BUNDLE}"; then
    echo "Error: failed to download ${BUNDLE} for ${VERSION}"
    exit 1
  fi
  if ! cosign verify-blob \
    --bundle "${WORK}/${BUNDLE}" \
    --certificate-identity "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/${VERSION}" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    "${WORK}/${CHECKSUMS}"; then
    echo "Error: signature verification failed for ${CHECKSUMS}"
    echo "If your cosign is old, update it and retry."
    exit 1
  fi
elif [ "$VERIFY_SIGNATURE" = "required" ]; then
  echo "Error: cosign not found, but GLASP_VERIFY_SIGNATURE=required"
  echo "Install cosign (https://docs.sigstore.dev/cosign/system_config/installation/) and retry."
  exit 1
else
  echo "Warning: cosign not found; skipping signature verification of ${CHECKSUMS}."
  echo "         Install cosign to verify that the release was built by ${REPO}."
fi

# Verify checksum
echo "Verifying checksum..."
EXPECTED="$(grep "  ${ARTIFACT}$" "${WORK}/${CHECKSUMS}" | cut -d ' ' -f 1)"
if [ -z "$EXPECTED" ]; then
  echo "Error: checksum not found for ${ARTIFACT}"
  exit 1
fi
ACTUAL="$($SHASUM "${WORK}/${ARTIFACT}" | cut -d ' ' -f 1)"
if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "Error: checksum mismatch"
  echo "  expected: $EXPECTED"
  echo "  actual:   $ACTUAL"
  exit 1
fi

# Extract and install
tar -xzf "${WORK}/${ARTIFACT}" -C "${WORK}" glasp
if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || {
    echo "Error: cannot create ${INSTALL_DIR} (permission denied)"
    echo "Choose a writable directory or run with sudo:"
    echo "  curl -fsSL https://takihito.github.io/glasp/install.sh | sudo GLASP_INSTALL_DIR=${INSTALL_DIR} sh"
    exit 1
  }
fi
if [ -w "$INSTALL_DIR" ]; then
  mv "${WORK}/glasp" "${INSTALL_DIR}/glasp"
else
  echo "Error: ${INSTALL_DIR} is not writable"
  echo "Choose a writable directory or run with sudo:"
  echo "  curl -fsSL https://takihito.github.io/glasp/install.sh | sudo GLASP_INSTALL_DIR=${INSTALL_DIR} sh"
  exit 1
fi
echo "Installed to ${INSTALL_DIR}/glasp"

# Check if INSTALL_DIR is in PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "Note: ${INSTALL_DIR} is not in your PATH."
    echo "Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

echo ""
echo "glasp ${VERSION} installed successfully!"
echo "Run 'glasp version' to verify."
