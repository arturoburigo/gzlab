#!/usr/bin/env sh
set -eu

repo="${GITLAB_TUI_REPO:-arturoburigo/gzlab}"
version="${GITLAB_TUI_VERSION:-latest}"
install_dir="${GITLAB_TUI_INSTALL_DIR:-$HOME/.local/bin}"
base_url="${GITLAB_TUI_BASE_URL:-https://github.com/$repo/releases/download}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os" in
  darwin) os="darwin" ;;
  linux) os="linux" ;;
  msys*|mingw*|cygwin*) os="windows" ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$version" = "latest" ]; then
  if command -v curl >/dev/null 2>&1; then
    version="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
  else
    echo "curl is required when GITLAB_TUI_VERSION=latest" >&2
    exit 1
  fi
fi
if [ -z "$version" ]; then
  echo "could not resolve release version" >&2
  exit 1
fi

archive="gzlab_${version}_${os}_${arch}.tar.gz"
if [ "$os" = "windows" ]; then
  archive="gzlab_${version}_${os}_${arch}.zip"
fi
url="$base_url/$version/$archive"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $url"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp/$archive"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$url" -O "$tmp/$archive"
else
  echo "curl or wget is required" >&2
  exit 1
fi

mkdir -p "$install_dir"
if [ "$os" = "windows" ]; then
  unzip -q "$tmp/$archive" -d "$tmp"
  bin="$(find "$tmp" -name 'gzlab.exe' -type f | head -n 1)"
  cp "$bin" "$install_dir/gzlab.exe"
else
  tar -xzf "$tmp/$archive" -C "$tmp"
  bin="$(find "$tmp" -name 'gzlab' -type f | head -n 1)"
  cp "$bin" "$install_dir/gzlab"
  chmod +x "$install_dir/gzlab"
fi

echo "Installed gzlab to $install_dir"
