#!/usr/bin/env python3
"""Export BAAI/bge-reranker-v2-m3 to ONNX for klimatsearch.

Not run by CI. Requires:
  pip install 'optimum[onnxruntime]' transformers torch

Writes models/bge-reranker-v2-m3/model.onnx and tokenizer.json.
Go scores query+Resource pairs (XLM-RoBERTa <s> q </s></s> p </s>).
"""
from __future__ import annotations

import shutil
from pathlib import Path

HF_ID = "BAAI/bge-reranker-v2-m3"
DEST = Path("models/bge-reranker-v2-m3")


def main() -> None:
    from optimum.onnxruntime import ORTModelForSequenceClassification
    from transformers import AutoTokenizer

    DEST.mkdir(parents=True, exist_ok=True)
    print(f"exporting {HF_ID} → {DEST}")
    model = ORTModelForSequenceClassification.from_pretrained(HF_ID, export=True)
    model.save_pretrained(DEST)
    tok = AutoTokenizer.from_pretrained(HF_ID)
    tok.save_pretrained(DEST)
    onnx = DEST / "model.onnx"
    if not onnx.exists():
        matches = list(DEST.rglob("model.onnx"))
        if matches:
            shutil.copy2(matches[0], onnx)
    if not onnx.exists():
        raise SystemExit(f"no model.onnx under {DEST}; check optimum output")
    print("ok:", onnx, "tokenizer.json:", (DEST / "tokenizer.json").exists())
    print("Next: KLIMAT_RERANKER=onnx and search with rerank=true")


if __name__ == "__main__":
    main()
