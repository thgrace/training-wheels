#!/bin/sh

set -eu

REPO="thgrace/training-wheels"
BIN_NAME="tw"

say() {
  printf '%s\n' "$*"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

download_to() {
  url=$1
  dest=$2

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
    return
  fi

  die "missing required downloader: install curl or wget"
}

detect_os() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    linux|darwin)
      printf '%s' "$os"
      ;;
    msys*|mingw*|cygwin*)
      die "Windows is supported via install.ps1; run the PowerShell installer instead"
      ;;
    *)
      die "unsupported operating system: $os"
      ;;
  esac
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64)
      printf 'amd64'
      ;;
    arm64|aarch64)
      printf 'arm64'
      ;;
    *)
      die "unsupported architecture: $arch"
      ;;
  esac
}

resolve_version() {
  if [ -z "${TW_VERSION:-}" ]; then
    printf 'latest'
    return
  fi

  case "$TW_VERSION" in
    latest|v*)
      printf '%s' "$TW_VERSION"
      ;;
    *)
      printf 'v%s' "$TW_VERSION"
      ;;
  esac
}

resolve_install_dir() {
  if [ -n "${TW_INSTALL_DIR:-}" ]; then
    printf '%s' "$TW_INSTALL_DIR"
    return
  fi

  if [ -z "${HOME:-}" ]; then
    die "HOME is not set; set TW_INSTALL_DIR to continue"
  fi

  printf '%s/.tw/bin' "$HOME"
}

calculate_sha256() {
  file=$1

  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi

  return 1
}

verify_checksum() {
  file=$1
  checksum_file=$2
  asset=$3

  expected=$(awk -v asset="$asset" '$2 == asset { print $1; exit }' "$checksum_file")
  if [ -z "$expected" ]; then
    warn "checksum entry for $asset was not found; skipping verification"
    return
  fi

  if ! actual=$(calculate_sha256 "$file"); then
    warn "no SHA-256 tool found; skipping verification"
    return
  fi

  if [ "$actual" != "$expected" ]; then
    die "checksum mismatch for $asset"
  fi

  say "Verified checksum for $asset."
}

install_binary() {
  src=$1
  dest_dir=$2
  dest_path=$dest_dir/$BIN_NAME

  if mkdir -p "$dest_dir" 2>/dev/null && [ -w "$dest_dir" ]; then
    if command -v install >/dev/null 2>&1; then
      install -m 0755 "$src" "$dest_path"
    else
      cp "$src" "$dest_path"
      chmod 0755 "$dest_path"
    fi
    return
  fi

  if ! command -v sudo >/dev/null 2>&1; then
    die "install directory is not writable: $dest_dir; rerun with sudo or set TW_INSTALL_DIR"
  fi

  sudo mkdir -p "$dest_dir"
  if command -v install >/dev/null 2>&1; then
    sudo install -m 0755 "$src" "$dest_path"
  else
    sudo cp "$src" "$dest_path"
    sudo chmod 0755 "$dest_path"
  fi
}

OS=$(detect_os)
ARCH=$(detect_arch)
VERSION=$(resolve_version)
INSTALL_DIR=$(resolve_install_dir)
ASSET="$BIN_NAME-$OS-$ARCH"

case "$VERSION" in
  latest)
    RELEASE_PATH="latest/download"
    VERSION_LABEL="latest"
    ;;
  *)
    RELEASE_PATH="download/$VERSION"
    VERSION_LABEL="$VERSION"
    ;;
esac

TMPDIR=$(mktemp -d "${TMPDIR:-/tmp}/tw-install-XXXXXX")
trap 'rm -rf "$TMPDIR"' EXIT INT TERM HUP

BIN_URL="https://github.com/$REPO/releases/$RELEASE_PATH/$ASSET"
CHECKSUM_URL="https://github.com/$REPO/releases/$RELEASE_PATH/checksums.txt"
BIN_TMP="$TMPDIR/$ASSET"
CHECKSUM_TMP="$TMPDIR/checksums.txt"

say "Installing $BIN_NAME ($OS/$ARCH) from GitHub release $VERSION_LABEL..."
download_to "$BIN_URL" "$BIN_TMP"

if download_to "$CHECKSUM_URL" "$CHECKSUM_TMP"; then
  verify_checksum "$BIN_TMP" "$CHECKSUM_TMP" "$ASSET"
else
  warn "could not download checksums.txt; continuing without verification"
fi

install_binary "$BIN_TMP" "$INSTALL_DIR"
say "Installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"

case ":${PATH:-}:" in
  *":$INSTALL_DIR:"*)
    ;;
  *)
    warn "$INSTALL_DIR is not on PATH"
    ;;
esac
