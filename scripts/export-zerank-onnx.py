#!/usr/bin/env python3
"""Export zeroentropy/zerank-1-small to a single-logit ONNX graph.

zerank-1-small is a 1.7B Qwen3 decoder (Apache-2.0). Official scoring is:

  chat(system=query, user=passage, add_generation_prompt=True)
  last-token logit of "Yes" / 5.0

This script wraps that into one float so klimatsearch can score it the same
way as BGE (outputs[0] is the relevance logit). Do not load zerank-2 (4B)
into the klimatsearch process.

Not run by CI. Needs ~16 GiB RAM to export. Do not use trust_remote_code:
ZeroEntropy's modeling_zeranker.py imports sentence_transformers; we load Qwen3
directly from the already-downloaded folder.

  uv run --with torch --with transformers --with onnx python scripts/export-zerank-onnx.py

Then:

  python scripts/quantize-onnx.py models/zerank-1-small
  KLIMAT_RERANKER=onnx KLIMAT_RERANKER_MODEL=zerank-1-small
"""
from __future__ import annotations

import shutil
from pathlib import Path

import torch

HF_ID = "zeroentropy/zerank-1-small"
DEST = Path("models/zerank-1-small")


class YesLogit(torch.nn.Module):
    def __init__(self, model: torch.nn.Module, yes_id: int) -> None:
        super().__init__()
        self.model = model
        self.yes_id = yes_id

    def forward(self, input_ids: torch.Tensor, attention_mask: torch.Tensor) -> torch.Tensor:
        out = self.model(
            input_ids=input_ids,
            attention_mask=attention_mask,
            use_cache=False,
        )
        logits = out.logits
        last = attention_mask.sum(dim=1) - 1
        batch = torch.arange(logits.size(0), device=logits.device)
        yes = logits[batch, last, self.yes_id] / 5.0
        return yes.unsqueeze(-1)


def chat(query: str, document: str) -> str:
    return (
        "<|im_start|>system\n"
        f"{query}<|im_end|>\n"
        "<|im_start|>user\n"
        f"{document}<|im_end|>\n"
        "<|im_start|>assistant\n"
    )


def main() -> None:
    from transformers import AutoTokenizer, Qwen3Config, Qwen3ForCausalLM

    DEST.mkdir(parents=True, exist_ok=True)
    src = DEST if (DEST / "model.safetensors").exists() else HF_ID
    print(f"exporting {src} → {DEST} (this is slow; no sentence_transformers)")
    tok = AutoTokenizer.from_pretrained(src, padding_side="right")
    tok.save_pretrained(DEST)
    # config.json auto_map points at modeling_zeranker.py (needs sentence_transformers).
    # ZEConfig is Qwen3Config; load the causal LM without remote code.
    config = Qwen3Config.from_pretrained(src)
    model = Qwen3ForCausalLM.from_pretrained(src, config=config, torch_dtype=torch.float32)
    model.eval()
    yes_id = tok.encode("Yes", add_special_tokens=False)[0]
    wrapped = YesLogit(model, yes_id)
    wrapped.eval()

    encoded = tok(chat("spånskiva", "Particle board"), return_tensors="pt")
    dummy = (encoded["input_ids"], encoded["attention_mask"])
    onnx_path = DEST / "model.onnx"
    torch.onnx.export(
        wrapped,
        dummy,
        str(onnx_path),
        input_names=["input_ids", "attention_mask"],
        output_names=["logits"],
        dynamic_axes={
            "input_ids": {0: "batch", 1: "seq"},
            "attention_mask": {0: "batch", 1: "seq"},
            "logits": {0: "batch"},
        },
        opset_version=17,
        dynamo=False,
    )
    if not onnx_path.exists():
        matches = list(DEST.rglob("model.onnx"))
        if matches:
            shutil.copy2(matches[0], onnx_path)
    if not onnx_path.exists():
        raise SystemExit(f"no model.onnx under {DEST}")
    print("ok:", onnx_path, "yes_id:", yes_id)
    print("Next: python scripts/quantize-onnx.py", DEST)
    print("Then: KLIMAT_RERANKER=onnx KLIMAT_RERANKER_MODEL=zerank-1-small")


if __name__ == "__main__":
    main()
