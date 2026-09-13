#!/usr/bin/env bash
# promptrouter installer. Downloads the right prebuilt binary for your machine
# and drops it in ~/.local/bin (or /usr/local/bin with sudo). No Go needed.
#
#   curl -fsSL https://raw.githubusercontent.com/taynotfound/promptrouter/main/install.sh | bash
#
set -euo pipefail

REPO="taynotfound/promptrouter"
BIN="route"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux|darwin) ;;
  *) echo "unsupported os: $os" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep -oE '"tag_name": *"[^"]+"' | head -1 | cut -d'"' -f4)
if [ -z "${tag:-}" ]; then
  echo "could not find a release. Build from source: go install github.com/$REPO/cmd/route@latest" >&2
  exit 1
fi

ver="${tag#v}"
url="https://github.com/$REPO/releases/download/$tag/promptrouter_${ver}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $BIN $tag ($os/$arch)..."
curl -fsSL "$url" -o "$tmp/pkg.tar.gz"
tar -xzf "$tmp/pkg.tar.gz" -C "$tmp"

# pick an install dir we can actually write to
if [ -w "/usr/local/bin" ]; then
  dest="/usr/local/bin"
else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
fi
install -m 0755 "$tmp/$BIN" "$dest/$BIN"

# drop a default config if none exists
cfg="$HOME/.config/promptrouter/models.yaml"
if [ ! -f "$cfg" ]; then
  mkdir -p "$(dirname "$cfg")"
  cp "$tmp/models.yaml" "$cfg" 2>/dev/null || true
  echo "wrote default config to $cfg"
fi

echo "installed $BIN to $dest"
case ":$PATH:" in
  *":$dest:"*) ;;
  *) echo "note: add $dest to your PATH" ;;
esac
echo "try: $BIN --explain \"rename a variable\""
