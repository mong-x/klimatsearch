#!/usr/bin/env bash
# Download production embedding weights for klimatsearch.
# Preferred model: codefuse-ai/F2LLM-v2-80M (hidden_size 320).
# Weights are not stored in git. Conversion to ONNX is required.
set -euo pipefail

if [[ -n "${CI:-}" || -n "${GITHUB_ACTIONS:-}" ]]; then
  echo "scripts/download-models.sh refuses to run in CI (weights are large and not required for tests)." >&2
  exit 1
fi

MODEL="${KLIMAT_EMBEDDING_MODEL:-f2llm-v2-80m}"
HF_ID="${F2LLM_HF_ID:-codefuse-ai/F2LLM-v2-80M}"
DEST="${KLIMAT_MODELS:-./models}/${MODEL}"
mkdir -p "$DEST"

echo "Downloading ${HF_ID} into ${DEST}"
if command -v huggingface-cli >/dev/null 2>&1; then
  huggingface-cli download "$HF_ID" --local-dir "$DEST"
elif command -v hf >/dev/null 2>&1; then
  hf download "$HF_ID" --local-dir "$DEST"
else
  echo "Install huggingface_hub (huggingface-cli) or set files manually under ${DEST}/" >&2
  echo "Expected layout:" >&2
  echo "  ${DEST}/model.onnx" >&2
  echo "  ${DEST}/tokenizer.json" >&2
  exit 1
fi

cat <<'EOF'

ONNX export is required. Example (Python, not run by this script):

  # pip install 'optimum[onnxruntime]' transformers
  # from optimum.onnxruntime import ORTModelForFeatureExtraction
  # model = ORTModelForFeatureExtraction.from_pretrained(
  #     "codefuse-ai/F2LLM-v2-80M", export=True)
  # model.save_pretrained("./models/f2llm-v2-80m")

Then:

  KLIMAT_EMBEDDER=onnx go run ./cmd/klimatsearch

EOF
