#!/usr/bin/env python3
"""Export codefuse-ai/F2LLM-v2-80M to ONNX for klimatsearch.

Not run by CI. Requires:
  pip install 'optimum[onnxruntime]' transformers torch

Writes models/f2llm-v2-80m/model.onnx (and copies tokenizer files if present).
Pooling is last-token/EOS in Go, not inside this file.
"""
from __future__ import annotations

import shutil
from pathlib import Path

HF_ID = "codefuse-ai/F2LLM-v2-80M"
DEST = Path("models/f2llm-v2-80m")


def main() -> None:
    from optimum.onnxruntime import ORTModelForFeatureExtraction
    from transformers import AutoTokenizer

    DEST.mkdir(parents=True, exist_ok=True)
    print(f"exporting {HF_ID} → {DEST}")
    model = ORTModelForFeatureExtraction.from_pretrained(HF_ID, export=True)
    model.save_pretrained(DEST)
    tok = AutoTokenizer.from_pretrained(HF_ID)
    tok.save_pretrained(DEST)
    onnx = DEST / "model.onnx"
    if not onnx.exists():
        # optimum sometimes writes model.onnx under a nested folder
        matches = list(DEST.rglob("model.onnx"))
        if matches:
            shutil.copy2(matches[0], onnx)
    if not onnx.exists():
        raise SystemExit(f"no model.onnx under {DEST}; check optimum output")
    print("ok:", onnx, "tokenizer.json:", (DEST / "tokenizer.json").exists())
    print("Next: set ONNXRUNTIME_LIB, wire Tokenizer in Go, KLIMAT_EMBEDDER=onnx")


if __name__ == "__main__":
    main()
