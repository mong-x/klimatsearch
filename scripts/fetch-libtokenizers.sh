#!/usr/bin/env bash
# Prebuilt HuggingFace tokenizers C library for daulet/tokenizers (CGO).
# Not used by CI (Fake embedder). Required for -tags tokenizers / make build locally.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/third_party/tokenizers"
VER="${TOKENIZERS_LIB_VERSION:-v1.27.0}"
mkdir -p "$DEST"
os="$(uname -s)"
arch="$(uname -m)"
case "$os-$arch" in
  Darwin-arm64) asset="libtokenizers.darwin-arm64.tar.gz" ;;
  Darwin-x86_64) asset="libtokenizers.darwin-x86_64.tar.gz" ;;
  Linux-x86_64|Linux-amd64) asset="libtokenizers.linux-amd64.tar.gz" ;;
  Linux-aarch64|Linux-arm64) asset="libtokenizers.linux-arm64.tar.gz" ;;
  *) echo "unsupported $os $arch — see https://github.com/daulet/tokenizers/releases" >&2; exit 1 ;;
esac
url="https://github.com/daulet/tokenizers/releases/download/${VER}/${asset}"
echo "fetching $url"
curl -fsSL "$url" | tar -xz -C "$DEST"
ls -lh "$DEST"/libtokenizers.*
echo "next: make test  # Makefile adds -tags tokenizers when the .a is present"
