#!/bin/sh
# install.sh — download and install the squad binary.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/0funct0ry/squad/main/scripts/install.sh | sh
#
# Pin a version with the SQUAD_VERSION env var:
#   curl -fsSL .../install.sh | SQUAD_VERSION=v1.2.3 sh
set -eu

REPO="0funct0ry/squad"
VERSION="${SQUAD_VERSION:-latest}"

log() { printf '%s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }

detect_os() {
  os=$(uname -s)
  case "$os" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    MINGW*|MSYS*|CYGWIN*)
      die "Windows is not supported by this script. Use Scoop (scoop install squad) or download a release archive from https://github.com/${REPO}/releases"
      ;;
    *) die "Unsupported OS: $os" ;;
  esac
}

detect_arch() {
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) die "Unsupported architecture: $arch" ;;
  esac
}

resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    [ -n "$tag" ] || die "could not resolve latest release version"
    echo "$tag"
  else
    echo "$VERSION"
  fi
}

checksum_verify() {
  file="$1"
  checksums="$2"
  line=$(grep " ${file}\$" "$checksums" || true)
  [ -n "$line" ] || die "no checksum entry found for ${file} in checksums.txt"

  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$(dirname "$file")" && echo "$line" | sha256sum -c -) || die "checksum verification failed for ${file}"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$(dirname "$file")" && echo "$line" | shasum -a 256 -c -) || die "checksum verification failed for ${file}"
  else
    die "neither sha256sum nor shasum is available; refusing to install unverified binary"
  fi
}

pick_install_dir() {
  if [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
  else
    dir="${HOME}/.local/bin"
    mkdir -p "$dir"
    echo "$dir"
  fi
}

main() {
  os=$(detect_os)
  arch=$(detect_arch)
  version=$(resolve_version)

  asset="squad_${os}_${arch}.tar.gz"
  base_url="https://github.com/${REPO}/releases/download/${version}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT INT TERM

  log "Downloading ${asset} (${version})..."
  curl -fsSL -o "${tmpdir}/${asset}" "${base_url}/${asset}" || die "failed to download ${asset}"
  curl -fsSL -o "${tmpdir}/checksums.txt" "${base_url}/checksums.txt" || die "failed to download checksums.txt"

  log "Verifying checksum..."
  checksum_verify "${tmpdir}/${asset}" "${tmpdir}/checksums.txt"

  log "Extracting..."
  tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"
  [ -f "${tmpdir}/squad" ] || die "extracted archive did not contain a squad binary"
  chmod +x "${tmpdir}/squad"

  install_dir=$(pick_install_dir)
  mv "${tmpdir}/squad" "${install_dir}/squad"
  log "Installed squad to ${install_dir}/squad"

  case ":${PATH}:" in
    *":${install_dir}:"*) ;;
    *) log "warning: ${install_dir} is not on your PATH. Add it to your shell profile." ;;
  esac

  "${install_dir}/squad" version
}

main "$@"
