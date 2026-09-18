#!/usr/bin/env python3
"""Dynamic INT8 quantize model.onnx → model.int8.onnx.

Usage:
  python scripts/quantize-onnx.py models/f2llm-v2-80m
  python scripts/quantize-onnx.py models/bge-reranker-v2-m3
  python scripts/quantize-onnx.py models/zerank-1-small

klimatsearch loads model.int8.onnx when KLIMAT_ONNX_QUANT=auto (default) or int8.
Not run by CI. Requires: pip install onnxruntime onnx
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dir", type=Path, help="model directory containing model.onnx")
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        default=None,
        help="output path (default: DIR/model.int8.onnx)",
    )
    args = parser.parse_args()
    src = args.dir / "model.onnx"
    if not src.exists():
        raise SystemExit(f"missing {src}")
    dst = args.output if args.output is not None else args.dir / "model.int8.onnx"

    from onnxruntime.quantization import QuantType, quantize_dynamic

    print(f"quantize {src} → {dst}")
    quantize_dynamic(
        model_input=str(src),
        model_output=str(dst),
        weight_type=QuantType.QInt8,
    )
    if not dst.exists():
        raise SystemExit(f"quantize wrote nothing at {dst}")
    print("ok:", dst, "bytes:", dst.stat().st_size)
    print("Start with KLIMAT_ONNX_QUANT=auto (default) or int8")


if __name__ == "__main__":
    try:
        main()
    except ImportError as e:
        print("pip install onnxruntime onnx  #", e, file=sys.stderr)
        raise SystemExit(1)
