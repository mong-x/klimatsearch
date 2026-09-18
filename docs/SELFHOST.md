# Self-host: embedder and reranker

This is the layout klimatsearch expects on a laptop, a VM, ECS, or EKS. Weights are not in git. The default `make run` binary uses the **fake** embedder and needs none of this.

## Canonical models

| Role | Model | Files klimatsearch reads | Env |
| --- | --- | --- | --- |
| **Embedder** | [codefuse-ai/F2LLM-v2-80M](https://huggingface.co/codefuse-ai/F2LLM-v2-80M), hidden size **320** | `models/f2llm-v2-80m/model.int8.onnx` (preferred) or `model.onnx`, plus `tokenizer.json` | `KLIMAT_EMBEDDER=onnx` |
| **Reranker (default)** | [BAAI/bge-reranker-v2-m3](https://huggingface.co/BAAI/bge-reranker-v2-m3) (0.6B encoder) | `models/bge-reranker-v2-m3/model.int8.onnx` or `model.onnx` (+ `model.onnx_data`), `tokenizer.json` | `KLIMAT_RERANKER=onnx` plus search `rerank=true` |
| **Reranker (quality)** | [zeroentropy/zerank-1-small](https://huggingface.co/zeroentropy/zerank-1-small) (1.7B Qwen3, Apache-2.0) | `models/zerank-1-small/model.int8.onnx` or `model.onnx`, `tokenizer.json` | `KLIMAT_RERANKER=onnx` `KLIMAT_RERANKER_MODEL=zerank-1-small` |

Reranker is optional. Default `KLIMAT_RERANKER=none` keeps FTS + sqlite-vec only. `onnx` loads the session at process start (same fail-closed rule as the embedder).

`KLIMAT_ONNX_QUANT` (`auto` \| `int8` \| `fp32`, default `auto`) picks the file: `auto` uses `model.int8.onnx` when present, otherwise `model.onnx`. On the 44-query golden set, **BGE-m3 INT8 is 44/44 Hit@1** and faster than BGE fp32. **Do not use zerank-1-small INT8** (Hit@1 collapsed). zerank fp32 is optional and slower with no Hit@1 gain.

Memory: F2LLM ~80M plus BGE-m3 ~0.6B — start at **4 GiB**. F2LLM + INT8 zerank-1-small — start at **8 GiB**. Do not load **zerank-2** (4B) into this process.

A different embedding model is a new directory under `--models` plus a re-ingest (dimension is stored in SQLite `meta`; a mismatch disables vector search).

## One-time prepare (any machine)

```bash
./scripts/fetch-libtokenizers.sh
make models
python scripts/export-f2llm-onnx.py
python scripts/export-bge-reranker-onnx.py   # or: python scripts/export-zerank-onnx.py
python scripts/quantize-onnx.py models/f2llm-v2-80m
python scripts/quantize-onnx.py models/bge-reranker-v2-m3
KLIMAT_RERANKER=onnx ./scripts/check-models.sh
```

macOS ONNX Runtime: `brew install onnxruntime`. Linux/AWS: `./scripts/fetch-onnxruntime.sh` then export the printed `ONNXRUNTIME_LIB`.

Expected tree:

```
models/f2llm-v2-80m/model.onnx           # ~300 MB, gitignored
models/f2llm-v2-80m/model.int8.onnx      # optional; KLIMAT_ONNX_QUANT=auto prefers this
models/f2llm-v2-80m/tokenizer.json
models/bge-reranker-v2-m3/model.onnx      # graph; gitignored
models/bge-reranker-v2-m3/model.onnx_data # ~2 GB weights, keep next to model.onnx
models/bge-reranker-v2-m3/model.int8.onnx
models/bge-reranker-v2-m3/tokenizer.json
# optional quality reranker:
models/zerank-1-small/model.onnx
models/zerank-1-small/model.int8.onnx
models/zerank-1-small/tokenizer.json
third_party/tokenizers/libtokenizers.a
# Linux:
third_party/onnxruntime/libonnxruntime.so
```

Build with tokenizers (Makefile adds the tag when the `.a` exists):

```bash
make build
export KLIMAT_EMBEDDER=onnx
export KLIMAT_RERANKER=onnx          # optional; default none
export KLIMAT_ONNX_QUANT=auto        # prefers model.int8.onnx
# export KLIMAT_RERANKER_MODEL=zerank-1-small
export ONNXRUNTIME_LIB=...          # if not in /usr/lib or Homebrew
rm -f data/klimat.db                # once, so vectors match this ONNX
./bin/klimatsearch --listen=:8080
```

`KLIMAT_EMBEDDER=onnx` **exits at start** if the resolved ONNX file (`model.int8.onnx` or `model.onnx`), `tokenizer.json`, or libonnxruntime is missing. Embedder `auto` falls back to fake when those files are absent — do not use embedder `auto` in production. `KLIMAT_RERANKER=onnx` is the same fail-closed rule for the reranker directory.

After ingest, search with vectors and rerank (REST defaults both **off**; MCP reranks whenever the process loaded `KLIMAT_RERANKER=onnx`):

```bash
curl -sS 'http://127.0.0.1:8080/api/search?q=spånskiva&lang=sv&vector=true&rerank=true'
```

## Docker / AWS

1. Prepare `models/` on a build machine (or CI with Hugging Face access). Do not download 300 MB on every task start.
2. Build a linux-amd64 binary with `fts5` and `tokenizers` (`./scripts/fetch-libtokenizers.sh` on Linux, then `make build`).
3. Put `libonnxruntime.so` next to the process or in the image (`./scripts/fetch-onnxruntime.sh`).
4. Mount models read-only. Embedder-only: **1–2 GiB**. Embedder + BGE: **4 GiB**. Embedder + zerank-1-small: **8 GiB**. Kind e2e’s `512Mi` is fake-embedder only.

Compose (models directory next to the compose file). **Reranker is on** (`KLIMAT_RERANKER=onnx`, BGE-m3, 4 GiB):

```bash
docker compose -f deploy/compose.onnx.yml up --build
# skip reranker:  KLIMAT_RERANKER=none docker compose -f deploy/compose.onnx.yml up --build
# zerank:        KLIMAT_RERANKER_MODEL=zerank-1-small  (raise mem_limit to 8g)
```

Kubernetes example with the same knobs: `deploy/k8s/deployment.onnx.yaml` (kind e2e still uses the fake `deployment.yaml`). Autoscale: `deploy/k8s/hpa.onnx.yaml` — replica-local SQLite only; see [AGENT-SELFHOST.md](AGENT-SELFHOST.md).

ECS/EKS sketch:

| Knob | Value |
| --- | --- |
| Command | `klimatsearch` (no `--demo-fixture`) |
| `KLIMAT_EMBEDDER` | `onnx` |
| `KLIMAT_RERANKER` | `onnx` (or `none`) |
| `KLIMAT_RERANKER_MODEL` | `bge-reranker-v2-m3` or `zerank-1-small` |
| `KLIMAT_ONNX_QUANT` | `auto` (loads `model.int8.onnx` when present) |
| `KLIMAT_MODELS` | `/models` |
| `KLIMAT_EMBEDDING_MODEL` | `f2llm-v2-80m` |
| `ONNXRUNTIME_LIB` | `/usr/local/lib/libonnxruntime.so` |
| Volume | EFS / EBS / image layer with `models/f2llm-v2-80m/` and optionally `models/bge-reranker-v2-m3/` or `models/zerank-1-small/` |
| Memory | 2 GiB embedder; 4 GiB with BGE; 8 GiB with zerank-1-small |
| Search | clients pass `vector=true` and `rerank=true` |

PVC for `data/klimat.db` if you do not want to re-embed ~230 rows on every cold start.

Production image (self-hosted, no Unkey/MPP):

```bash
docker build --build-arg GO_TAGS="fts5 tokenizers" -t klimatsearch:onnx .
```

The default Dockerfile runtime image is Debian slim **without** libonnxruntime or weights. Copy `libonnxruntime.so` and mount `models/` yourself, or add a private image layer. Do not commit weights. `deploy/compose.onnx.yml` replaces the image `CMD` (`--embedder=fake --demo-fixture`) with `--ingest-on-start` so `KLIMAT_EMBEDDER` / `KLIMAT_RERANKER` from the compose env take effect.

## Verify

```bash
./scripts/check-models.sh
make eval-golden   # testdata/golden-queries.json; needs F2LLM-ingested data/klimat.db
curl -sS http://127.0.0.1:8080/healthz
# first ONNX ingest logs embedding work; then:
curl -sS 'http://127.0.0.1:8080/api/search?q=betong&lang=sv&vector=true&rerank=true' | head
```

If start fails, the error names the missing file or `onnxruntime init` — set `ONNXRUNTIME_LIB` to the `.so` / `.dylib`.
