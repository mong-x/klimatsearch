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

quant="${KLIMAT_ONNX_QUANT:-auto}"

pick_onnx() {
  local d="$1"
  case "${quant}" in
    int8)
      need "${d}/model.int8.onnx"
      ;;
    fp32|none)
      need "${d}/model.onnx"
      ;;
    *)
      if [[ -f "${d}/model.int8.onnx" ]]; then
        echo "ok  ${d}/model.int8.onnx  (auto)"
      elif [[ -f "${d}/model.onnx" ]]; then
        echo "ok  ${d}/model.onnx  (auto)"
      else
        echo "MISSING  ${d}/model.onnx (or model.int8.onnx)" >&2
        ok=1
      fi
      ;;
  esac
  need "${d}/tokenizer.json"
}

echo "embedder model: ${DIR}  quant=${quant}"
pick_onnx "${DIR}"

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
  pick_onnx "${RDIR}"
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
echo "  KLIMAT_EMBEDDER=onnx KLIMAT_RERANKER=${rerank} KLIMAT_ONNX_QUANT=${quant} ONNXRUNTIME_LIB=${ort} ./bin/klimatsearch"
echo "Search with vector=true (and rerank=true if ONNX reranker is loaded)."
