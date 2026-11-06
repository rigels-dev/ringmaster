#!/bin/sh
# Install Ringmaster on Linux or macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/rigels-dev/ringmaster/main/install.sh | sh
#
# Environment:
#   RINGMASTER_VERSION   release to install, e.g. v0.2.0-bellatrix (default: latest)
#   RINGMASTER_HOME      install directory (default: ~/.local/share/ringmaster)
#   RINGMASTER_BIN_DIR   where the `ringmaster` command is linked (default: ~/.local/bin)
#   RINGMASTER_ARCHIVE   install from a local release archive instead of downloading
#   RINGMASTER_BASE_URL  download from a mirror laid out like GitHub releases
set -eu

REPO="rigels-dev/ringmaster"
HOME_DIR="${RINGMASTER_HOME:-$HOME/.local/share/ringmaster}"
BIN_DIR="${RINGMASTER_BIN_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'ringmaster install: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported system $(uname -s); Ringmaster runs on Linux and macOS" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m)" ;;
esac

fetch() { # fetch <url> <file>
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    fail "curl or wget is required"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

if [ -n "${RINGMASTER_ARCHIVE:-}" ]; then
  archive="$RINGMASTER_ARCHIVE"
  [ -f "$archive" ] || fail "no such archive: $archive"
else
  version="${RINGMASTER_VERSION:-}"
  if [ -z "$version" ]; then
    fetch "https://api.github.com/repos/$REPO/releases/latest" "$tmp/latest.json"
    version="$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$tmp/latest.json" | head -n1)"
    [ -n "$version" ] || fail "could not find the latest release"
  fi
  name="ringmaster-$version-$os-$arch.tar.gz"
  base="${RINGMASTER_BASE_URL:-https://github.com/$REPO/releases/download}/$version"
  say "Downloading Ringmaster $version for $os/$arch…"
  fetch "$base/$name" "$tmp/$name" || fail "download failed: $base/$name"
  fetch "$base/ringmaster-$version-checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums"
  expected="$(grep " $name\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
  [ -n "$expected" ] || fail "no checksum listed for $name"
  [ "$(sha256 "$tmp/$name")" = "$expected" ] || fail "checksum mismatch for $name; not installing"
  archive="$tmp/$name"
fi

# Unpack next to the current install, then swap it in
mkdir -p "$tmp/unpack"
tar -xzf "$archive" -C "$tmp/unpack"
src="$(find "$tmp/unpack" -mindepth 1 -maxdepth 1 -type d | head -n1)"
[ -x "$src/bin/ringmaster" ] || fail "archive does not contain bin/ringmaster"

mkdir -p "$(dirname "$HOME_DIR")" "$BIN_DIR"
rm -rf "$HOME_DIR.new"
cp -R "$src" "$HOME_DIR.new"
rm -rf "$HOME_DIR"
mv "$HOME_DIR.new" "$HOME_DIR"
ln -sf "$HOME_DIR/bin/ringmaster" "$BIN_DIR/ringmaster"

say "Installed $("$HOME_DIR/bin/ringmaster" --version) to $HOME_DIR"
say "Command: $BIN_DIR/ringmaster"

# What the tool needs at run time
if ! command -v git >/dev/null 2>&1; then
  say "Note: git 2.38 or newer is required."
fi
if command -v node >/dev/null 2>&1; then
  major="$(node --version | sed 's/^v\([0-9]*\).*/\1/')"
  [ "$major" -ge 22 ] 2>/dev/null || say "Note: the web UI needs Node.js 22 or newer (found $(node --version))."
else
  say "Note: the web UI needs Node.js 22 or newer; the CLI works without it."
fi
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) say "Add $BIN_DIR to your PATH, e.g. echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.profile" ;;
esac
say "Next: ringmaster config   (set up providers and agents)"
