#!/bin/sh
# AGI OS (agios) installer for macOS and Linux
# Usage: curl -fsSL https://install.agios.sh/install.sh | sh
set -e

REPO="agios-sh/agios"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="agios"

main() {
    check_dependencies
    detect_platform
    fetch_latest_version
    download_and_install
    verify_installation
}

check_dependencies() {
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        error "curl or wget is required but neither was found"
    fi
    if ! command -v tar >/dev/null 2>&1; then
        error "tar is required but was not found"
    fi
}

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) ;;
        linux) ;;
        *)
            error "Unsupported operating system: $OS (supported: darwin, linux)"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)
            error "Unsupported architecture: $ARCH (supported: amd64, arm64)"
            ;;
    esac

    info "Detected platform: ${OS}/${ARCH}"
}

fetch_latest_version() {
    info "Fetching latest release..."
    RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        RELEASE_JSON=$(curl -fsSL "$RELEASE_URL") || error "Failed to fetch latest release from GitHub"
    else
        RELEASE_JSON=$(wget -qO- "$RELEASE_URL") || error "Failed to fetch latest release from GitHub"
    fi

    # Extract tag_name from JSON without jq dependency
    TAG=$(echo "$RELEASE_JSON" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

    if [ -z "$TAG" ]; then
        error "Could not determine latest release version"
    fi

    info "Latest version: ${TAG}"
}

download_and_install() {
    ARCHIVE="${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${ARCHIVE}..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "$DOWNLOAD_URL" || error "Failed to download ${DOWNLOAD_URL}"
    else
        wget -qO "${TMPDIR}/${ARCHIVE}" "$DOWNLOAD_URL" || error "Failed to download ${DOWNLOAD_URL}"
    fi

    info "Extracting..."
    tar xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

    if [ ! -f "${TMPDIR}/${BINARY_NAME}" ]; then
        error "Binary not found in archive"
    fi

    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        info "Elevated permissions required to install to ${INSTALL_DIR}"
        sudo mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi
}

verify_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        INSTALLED_VERSION=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        info "Successfully installed ${BINARY_NAME} ${INSTALLED_VERSION}"
        info "Run 'agios init' to get started"
    else
        warn "${BINARY_NAME} was installed to ${INSTALL_DIR} but is not on your PATH"
        warn "Add ${INSTALL_DIR} to your PATH, then run 'agios init' to get started"
    fi
}

info() {
    printf "\033[1;32m==>\033[0m %s\n" "$1"
}

warn() {
    printf "\033[1;33mwarning:\033[0m %s\n" "$1" >&2
}

error() {
    printf "\033[1;31merror:\033[0m %s\n" "$1" >&2
    exit 1
}

main
