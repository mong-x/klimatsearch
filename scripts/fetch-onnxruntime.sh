#!/usr/bin/env bash
# Fetch a CPU ONNX Runtime shared library for this OS/arch.
# macOS: prefer `brew install onnxruntime`. This script is for Linux (AWS, Docker).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${ONNXRUNTIME_DIR:-$ROOT/third_party/onnxruntime}"
VER="${ONNXRUNTIME_VERSION:-1.20.1}"
mkdir -p "$DEST"
os="$(uname -s)"
arch="$(uname -m)"
case "$os-$arch" in
  Linux-x86_64|Linux-amd64) pack="onnxruntime-linux-x64-${VER}.tgz" ;;
  Linux-aarch64|Linux-arm64) pack="onnxruntime-linux-aarch64-${VER}.tgz" ;;
  Darwin-arm64)
    echo "on macOS use: brew install onnxruntime" >&2
    echo "then: export ONNXRUNTIME_LIB=/opt/homebrew/lib/libonnxruntime.dylib" >&2
    exit 1
    ;;
  Darwin-x86_64)
    echo "on macOS use: brew install onnxruntime" >&2
    echo "then: export ONNXRUNTIME_LIB=/usr/local/lib/libonnxruntime.dylib" >&2
    exit 1
    ;;
  *) echo "unsupported $os $arch" >&2; exit 1 ;;
esac
url="https://github.com/microsoft/onnxruntime/releases/download/v${VER}/${pack}"
echo "fetching $url"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" | tar -xz -C "$tmp"
lib="$(find "$tmp" -name 'libonnxruntime.so*' | head -n 1)"
if [[ -z "$lib" ]]; then
  echo "no libonnxruntime.so in archive" >&2
  exit 1
fi
cp -a "$(dirname "$lib")"/libonnxruntime.so* "$DEST/"
so="$(ls "$DEST"/libonnxruntime.so | head -n 1)"
echo "ok  $so"
echo "export ONNXRUNTIME_LIB=$so"
