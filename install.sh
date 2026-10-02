#!/usr/bin/env sh
# Install portcheck from the latest GitHub release.
#
#   curl -fsSL https://github.com/aman-void/portcheck/releases/latest/download/install.sh | sh
#
# Pins a specific version with PORTCHECK_VERSION:
#
#   PORTCHECK_VERSION=v1.0.0 curl -fsSL .../install.sh | sh
#
# Environment:
#   PORTCHECK_VERSION  release tag to install (default: latest)
#   PORTCHECK_INSTALL  install directory   (default: $HOME/.local/bin)
set -eu

REPO="aman-void/portcheck"
BIN="portcheck"

die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required but not found"; }

# --- 1. Work out which of the six published builds we want -------------------

os=$(uname -s)
arch=$(uname -m)

case "$os" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS '$os'. Build from source: go install github.com/$REPO/cmd/$BIN@latest" ;;
esac

case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture '$arch'. Build from source: go install github.com/$REPO/cmd/$BIN@latest" ;;
esac

# Accept either the tag form (v1.0.0-rc.1) or the bare version (1.0.0-rc.1).
# Release archive names carry the bare form, so strip a leading v before use.
version=${PORTCHECK_VERSION:-latest}
[ "$version" = "latest" ] || version=${version#v}

if [ -n "${PORTCHECK_BASE_URL:-}" ]; then
  # Override for mirrors, air-gapped installs, and testing. Validate it, so a
  # stray or mistyped value fails here instead of producing a nonsense URL.
  case ${PORTCHECK_BASE_URL} in
    http://*|https://*) base=${PORTCHECK_BASE_URL%/} ;;
    *) die "PORTCHECK_BASE_URL must start with http:// or https://, got '$PORTCHECK_BASE_URL'" ;;
  esac
elif [ "$version" = "latest" ]; then
  # GitHub excludes prereleases from releases/latest, so latest resolves to the
  # newest stable release, or fails when only prereleases exist.
  base="https://github.com/$REPO/releases/latest/download"
  version=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$base/$BIN" 2>/dev/null | sed 's|.*/tag/||' || echo unknown)
  if [ "$version" = "unknown" ] || [ -z "$version" ]; then
    die "no published stable release found at $base
If you are installing a prerelease, pin it explicitly:
  PORTCHECK_VERSION=v1.0.0-rc.1 <install command>"
  fi
  version=${version#v}
else
  base="https://github.com/$REPO/releases/download/v$version"
fi

if [ "$os" = "windows" ]; then
  archive="${BIN}_${version}_${os}_${arch}.zip"
else
  archive="${BIN}_${version}_${os}_${arch}.tar.gz"
fi

printf 'portcheck %s for %s/%s\n' "$version" "$os" "$arch"

# --- 2. Fetch the archive and the checksum manifest --------------------------

printf 'downloading %s\n' "$archive"

need curl
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$base/$archive" -o "$tmp/$archive" || die "download failed: $base/$archive"
curl -fsSL "$base/SHA256SUMS"      -o "$tmp/SHA256SUMS" || die "download failed: $base/SHA256SUMS"

# --- 3. Verify before executing anything -------------------------------------

want=$(awk -v f="$archive" '$2 == f || $2 == "*"f { print $1 }' "$tmp/SHA256SUMS")
[ -n "$want" ] || die "$archive is not listed in SHA256SUMS"

# Use the system tool when available; fall back to a pure-shell comparison so
# the script also works where sha256sum and shasum are both missing.
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
  got=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
else
  got=$(openssl dgst -sha256 "$tmp/$archive" | sed 's/.*= //')
fi

[ "$got" = "$want" ] || die "checksum mismatch for $archive
  expected $want
  actual   $got
Refusing to install an unverified binary."

# --- 4. Unpack and install ---------------------------------------------------

extract=tar
[ "$os" = "windows" ] && { need unzip; extract=unzip; }
"$extract" -xf "$tmp/$archive" -C "$tmp" || die "could not unpack $archive"

src=$(find "$tmp" -type f -name "$BIN" | head -n 1)
[ -n "$src" ] || die "$BIN not found inside $archive"

dest=${PORTCHECK_INSTALL:-$HOME/.local/bin}
mkdir -p "$dest"
chmod 0755 "$src"
mv -f "$src" "$dest/$BIN" || die "could not write to $dest (set PORTCHECK_INSTALL to a writable path)"

printf '\ninstalled %s -> %s\n' "$version" "$dest/$BIN"

case ":$PATH:" in
  *":$dest:"*) "$dest/$BIN" --version ;;
  *) printf '\n%s is not on your PATH. Add this to your shell profile:\n\n  export PATH="%s:$PATH"\n\n' "$dest" "$dest" ;;
esac