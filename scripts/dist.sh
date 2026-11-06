#!/usr/bin/env bash
# Build release archives for every supported platform into dist/.
#
#   scripts/dist.sh <version>        e.g. scripts/dist.sh v0.2.0-bellatrix
#
# Each archive unpacks to ringmaster-<version>-<os>-<arch>/ with the layout
# the launcher looks for in a packaged install:
#   bin/ringmaster
#   share/ringmaster/web/      (the built web UI, platform independent)
#   README.md LICENSE ringmaster.example.toml
# The web UI must already be built (make web).
set -euo pipefail

version="${1:?usage: scripts/dist.sh <version>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
web="$root/web/.output"
out="$root/dist"
targets="${TARGETS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64}"

if [ ! -f "$web/server/index.mjs" ]; then
  echo "web UI not built: run 'make web' first" >&2
  exit 1
fi

rm -rf "$out"
mkdir -p "$out"
cd "$root"

for target in $targets; do
  os="${target%/*}"
  arch="${target#*/}"
  name="ringmaster-$version-$os-$arch"
  stage="$out/$name"

  mkdir -p "$stage/bin" "$stage/share/ringmaster"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X ringmaster/internal/version.Version=$version" \
    -o "$stage/bin/ringmaster" ./cmd/ringmaster
  cp -R "$web" "$stage/share/ringmaster/web"
  cp README.md LICENSE ringmaster.example.toml "$stage/"

  tar -C "$out" -czf "$out/$name.tar.gz" "$name"
  rm -rf "$stage"
  echo "built $name.tar.gz"
done

# Checksums for install.sh and the Homebrew formula
if command -v sha256sum >/dev/null; then sum="sha256sum"; else sum="shasum -a 256"; fi
(cd "$out" && $sum ringmaster-*.tar.gz > "ringmaster-$version-checksums.txt")
echo "wrote ringmaster-$version-checksums.txt"
