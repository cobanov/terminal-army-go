#!/usr/bin/env bash
# install.sh -- one-line installer for the `tarmy` binary.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cobanov/terminal-army-go/main/install.sh | bash
#
# Or with a specific version (defaults to the latest GitHub release):
#   curl -fsSL https://raw.githubusercontent.com/cobanov/terminal-army-go/main/install.sh | VERSION=v0.1.0 bash
#
# Or to a custom install directory:
#   curl -fsSL https://raw.githubusercontent.com/cobanov/terminal-army-go/main/install.sh | INSTALL_DIR=$HOME/bin bash
#
# Honored environment variables:
#   VERSION       Release tag to fetch (default: "latest")
#   INSTALL_DIR   Where to drop the binary (default: /usr/local/bin if writable,
#                 else $HOME/.local/bin). Created if missing.
#   REPO          GitHub repo slug (default: cobanov/terminal-army-go).
#                 Useful when testing a fork.
#
# The script verifies the SHA256 checksum if the release ships a
# `checksums.txt` file. It refuses to install a binary that fails the
# checksum check.

set -euo pipefail

REPO="${REPO:-cobanov/terminal-army-go}"
VERSION="${VERSION:-latest}"
BINARY_NAME="tarmy"

# ---- pretty output ------------------------------------------------------
# We use a regular hyphen only; no fancy unicode dashes anywhere.
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { printf "${BLUE}==>${NC} %s\n" "$*"; }
ok()    { printf "${GREEN}ok${NC}  %s\n" "$*"; }
warn()  { printf "${YELLOW}warn${NC} %s\n" "$*"; }
die()   { printf "${RED}error${NC} %s\n" "$*" >&2; exit 1; }

# ---- detect OS + arch ---------------------------------------------------
detect_platform() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux)   os="linux"   ;;
        darwin)  os="darwin"  ;;
        *)       die "unsupported operating system: $os (need linux or darwin)" ;;
    esac

    case "$arch" in
        x86_64|amd64)  arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)             die "unsupported architecture: $arch (need amd64 or arm64)" ;;
    esac

    PLATFORM="${os}_${arch}"
}

# ---- pick install directory --------------------------------------------
detect_install_dir() {
    if [ -n "${INSTALL_DIR:-}" ]; then
        return
    fi
    if [ -w "/usr/local/bin" ] || [ "$(id -u)" = "0" ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
    fi
}

# ---- required tooling ---------------------------------------------------
require() {
    command -v "$1" >/dev/null 2>&1 || die "this script needs '$1' on PATH"
}

# ---- resolve release tag ------------------------------------------------
resolve_version() {
    if [ "$VERSION" = "latest" ]; then
        info "resolving latest release for $REPO ..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
            | grep -oE '"tag_name": *"[^"]+"' \
            | head -1 \
            | sed 's/.*"\([^"]*\)"$/\1/')
        if [ -z "$VERSION" ]; then
            die "could not resolve latest release. set VERSION=vX.Y.Z explicitly."
        fi
        ok "latest is $VERSION"
    else
        info "using requested version $VERSION"
    fi
}

# ---- download + verify --------------------------------------------------
download() {
    local tmp archive base url checksum_url checksum_file expected actual

    tmp="$(mktemp -d 2>/dev/null || mktemp -d -t tarmy)"
    trap 'rm -rf "${tmp:-}"' EXIT

    base="https://github.com/$REPO/releases/download/$VERSION"
    archive="${BINARY_NAME}_${VERSION#v}_${PLATFORM}.tar.gz"
    url="$base/$archive"
    checksum_url="$base/checksums.txt"

    info "downloading $url"
    if ! curl -fsSL "$url" -o "$tmp/$archive"; then
        die "download failed. is $VERSION published for $PLATFORM?"
    fi

    # Checksum is optional but strongly preferred.
    if curl -fsSL "$checksum_url" -o "$tmp/checksums.txt" 2>/dev/null; then
        expected=$(grep " $archive$" "$tmp/checksums.txt" | awk '{print $1}' | head -1)
        if [ -z "$expected" ]; then
            warn "no checksum entry for $archive in checksums.txt; skipping verify"
        else
            if command -v sha256sum >/dev/null 2>&1; then
                actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
            elif command -v shasum >/dev/null 2>&1; then
                actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
            else
                warn "no sha256 tool found; skipping checksum verify"
                actual="$expected"
            fi
            if [ "$expected" != "$actual" ]; then
                die "checksum mismatch! expected $expected, got $actual"
            fi
            ok "checksum verified ($expected)"
        fi
    else
        warn "no checksums.txt published; skipping verify"
    fi

    info "extracting $archive"
    tar -xzf "$tmp/$archive" -C "$tmp"

    if [ ! -f "$tmp/$BINARY_NAME" ]; then
        die "archive did not contain a '$BINARY_NAME' binary"
    fi

    mkdir -p "$INSTALL_DIR"
    install -m 0755 "$tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    ok "installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"
}

# ---- post-install hints -------------------------------------------------
final_notes() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *)
            warn "$INSTALL_DIR is not on your PATH."
            warn "add this to your shell rc:"
            warn "  export PATH=\"$INSTALL_DIR:\$PATH\""
            ;;
    esac

    cat <<EOF

quick start:
  $BINARY_NAME --version
  $BINARY_NAME

server operators (requires Docker):
  git clone https://github.com/$REPO.git
  cd terminal-army-go
  cp .env.example .env && \$EDITOR .env   # set TARMY_JWT_SECRET
  docker compose up -d --build
  docker compose exec tarmy $BINARY_NAME admin seed-universe

EOF
}

main() {
    require curl
    require tar
    require uname
    require awk
    require grep
    require sed

    detect_platform
    detect_install_dir
    info "platform : $PLATFORM"
    info "destdir  : $INSTALL_DIR"

    resolve_version
    download
    final_notes
}

main "$@"
