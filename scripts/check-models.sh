#!/usr/bin/env bash
# Verify the production embedder layout before start. Does not download.
# Usage: scripts/check-models.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
MODEL="${KLIMAT_EMBEDDING_MODEL:-f2llm-v2-80m}"
MODELS="${KLIMAT_MODELS:-./models}"
DIR="${MODELS}/${MODEL}"
ok=0

need() {
  if [[ -f "$1" ]]; then
    echo "ok  $1"
  else
    echo "MISSING  $1" >&2
    ok=1
  fi
}

echo "embedder model: ${DIR}"
need "${DIR}/model.onnx"
need "${DIR}/tokenizer.json"

a="${ROOT}/third_party/tokenizers/libtokenizers.a"
if [[ -f "$a" ]]; then
  echo "ok  $a"
else
  echo "MISSING  $a  (run ./scripts/fetch-libtokenizers.sh, then make build)" >&2
  ok=1
fi

ort="${ONNXRUNTIME_LIB:-}"
if [[ -z "$ort" ]]; then
  for p in \
    /opt/homebrew/lib/libonnxruntime.dylib \
    /usr/local/lib/libonnxruntime.dylib \
    /usr/lib/libonnxruntime.so \
    /usr/local/lib/libonnxruntime.so
  do
    if [[ -f "$p" ]]; then ort="$p"; break; fi
  done
fi
if [[ -n "$ort" && -f "$ort" ]]; then
  echo "ok  ONNX Runtime $ort"
else
  echo "MISSING  libonnxruntime  (set ONNXRUNTIME_LIB or run ./scripts/fetch-onnxruntime.sh)" >&2
  ok=1
fi

rerank="${KLIMAT_RERANKER:-none}"
RDIR="${MODELS}/${KLIMAT_RERANKER_MODEL:-bge-reranker-v2-m3}"
echo "reranker: ${rerank} (${RDIR})"
if [[ "${rerank}" == "onnx" ]]; then
  need "${RDIR}/model.onnx"
  need "${RDIR}/tokenizer.json"
  if [[ -f "${RDIR}/model.onnx_data" ]]; then
    echo "ok  ${RDIR}/model.onnx_data"
  fi
fi

echo
if [[ "$ok" -ne 0 ]]; then
  echo "layout incomplete. Canonical embedder is codefuse-ai/F2LLM-v2-80M (320-d)." >&2
  echo "docs/SELFHOST.md" >&2
  exit 1
fi
echo "layout ok. Start with:"
echo "  KLIMAT_EMBEDDER=onnx KLIMAT_RERANKER=${rerank} ONNXRUNTIME_LIB=${ort} ./bin/klimatsearch"
echo "Search with vector=true (and rerank=true if ONNX reranker is loaded)."
