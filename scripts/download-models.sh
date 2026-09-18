#!/usr/bin/env bash
# Download production embedding weights for klimatsearch.
# Preferred model: codefuse-ai/F2LLM-v2-80M (hidden_size 320).
# Weights are not stored in git. Conversion to ONNX is required.
set -euo pipefail

if [[ -n "${CI:-}" || -n "${GITHUB_ACTIONS:-}" ]]; then
  echo "scripts/download-models.sh refuses to run in CI (weights are large and not required for tests)." >&2
  exit 1
fi

download_hf() {
  local hf_id="$1" dest="$2"
  mkdir -p "$dest"
  echo "Downloading ${hf_id} into ${dest}"
  if command -v huggingface-cli >/dev/null 2>&1; then
    huggingface-cli download "$hf_id" --local-dir "$dest"
  elif command -v hf >/dev/null 2>&1; then
    hf download "$hf_id" --local-dir "$dest"
  elif command -v uv >/dev/null 2>&1; then
    uv run --with huggingface_hub python -c "from huggingface_hub import snapshot_download; snapshot_download('$hf_id', local_dir='$dest')"
  else
    echo "Install huggingface_hub (huggingface-cli) or uv, or copy files under ${dest}/" >&2
    return 1
  fi
}

MODEL="${KLIMAT_EMBEDDING_MODEL:-f2llm-v2-80m}"
HF_ID="${F2LLM_HF_ID:-codefuse-ai/F2LLM-v2-80M}"
DEST="${KLIMAT_MODELS:-./models}/${MODEL}"
RERANK_MODEL="${KLIMAT_RERANKER_MODEL:-bge-reranker-v2-m3}"
RERANK_DEST="${KLIMAT_MODELS:-./models}/${RERANK_MODEL}"
case "${RERANK_MODEL}" in
  zerank*|qwen*)
    RERANK_HF="${ZERANK_HF_ID:-zeroentropy/zerank-1-small}"
    ;;
  *)
    RERANK_HF="${BGE_RERANK_HF_ID:-BAAI/bge-reranker-v2-m3}"
    ;;
esac

download_hf "$HF_ID" "$DEST"
download_hf "$RERANK_HF" "$RERANK_DEST"

cat <<'EOF'

ONNX export is required. Example (Python, not run by this script):

  # pip install 'optimum[onnxruntime]' transformers
  # from optimum.onnxruntime import ORTModelForFeatureExtraction
  # model = ORTModelForFeatureExtraction.from_pretrained(
  #     "codefuse-ai/F2LLM-v2-80M", export=True)
  # model.save_pretrained("./models/f2llm-v2-80m")

Then export ONNX (refuses to run here so you can review the Python env):

  python scripts/export-f2llm-onnx.py
  python scripts/export-bge-reranker-onnx.py   # default reranker
  # or: python scripts/export-zerank-onnx.py  # zerank-1-small (Apache, 1.7B)
  python scripts/quantize-onnx.py models/f2llm-v2-80m
  python scripts/quantize-onnx.py models/bge-reranker-v2-m3

See docs/SELFHOST.md for layout, libonnxruntime, KLIMAT_RERANKER=onnx, and KLIMAT_ONNX_QUANT.

EOF
