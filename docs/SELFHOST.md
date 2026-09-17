# Self-host: embedder and reranker

This is the layout klimatsearch expects on a laptop, a VM, ECS, or EKS. Weights are not in git. The default `make run` binary uses the **fake** embedder and needs none of this.

## Canonical models

| Role | Model | Files klimatsearch reads | Env |
| --- | --- | --- | --- |
| **Embedder** | [codefuse-ai/F2LLM-v2-80M](https://huggingface.co/codefuse-ai/F2LLM-v2-80M), hidden size **320** | `models/f2llm-v2-80m/model.onnx` and `tokenizer.json` | `KLIMAT_EMBEDDER=onnx` |
| **Reranker** | [BAAI/bge-reranker-v2-m3](https://huggingface.co/BAAI/bge-reranker-v2-m3) (multilingual cross-encoder) | `models/bge-reranker-v2-m3/model.onnx` and `tokenizer.json` | `KLIMAT_RERANKER=onnx` plus search `rerank=true` |

Reranker is optional. Default `KLIMAT_RERANKER=none` keeps FTS + sqlite-vec only. `onnx` loads the session at process start (same fail-closed rule as the embedder). Memory: F2LLM ~80M plus BGE-m3 reranker ~0.6B — start at **4 GiB** if both are on.

A different embedding model is a new directory under `--models` plus a re-ingest (dimension is stored in SQLite `meta`; a mismatch disables vector search).

## One-time prepare (any machine)

```bash
./scripts/fetch-libtokenizers.sh
make models
python scripts/export-f2llm-onnx.py
python scripts/export-bge-reranker-onnx.py
KLIMAT_RERANKER=onnx ./scripts/check-models.sh
```

macOS ONNX Runtime: `brew install onnxruntime`. Linux/AWS: `./scripts/fetch-onnxruntime.sh` then export the printed `ONNXRUNTIME_LIB`.

Expected tree:

```
models/f2llm-v2-80m/model.onnx           # ~300 MB, gitignored
models/f2llm-v2-80m/tokenizer.json
models/bge-reranker-v2-m3/model.onnx     # ~1 GB, gitignored
models/bge-reranker-v2-m3/tokenizer.json
third_party/tokenizers/libtokenizers.a
# Linux:
third_party/onnxruntime/libonnxruntime.so
```

Build with tokenizers (Makefile adds the tag when the `.a` exists):

```bash
make build
export KLIMAT_EMBEDDER=onnx
export KLIMAT_RERANKER=onnx          # optional; default none
export ONNXRUNTIME_LIB=...          # if not in /usr/lib or Homebrew
rm -f data/klimat.db                # once, so vectors match this ONNX
./bin/klimatsearch --listen=:8080
```

`KLIMAT_EMBEDDER=onnx` **exits at start** if `model.onnx`, `tokenizer.json`, or libonnxruntime is missing. `auto` falls back to fake when the ONNX file is absent — do not use `auto` in production.

After ingest, search with vectors:

```bash
curl -sS 'http://127.0.0.1:8080/api/search?q=spånskiva&lang=sv&vector=true&rerank=true'
```

`vector` defaults to false (FTS only).

## Docker / AWS

1. Prepare `models/` on a build machine (or CI with Hugging Face access). Do not download 300 MB on every task start.
2. Build a linux-amd64 binary with `fts5` and `tokenizers` (`./scripts/fetch-libtokenizers.sh` on Linux, then `make build`).
3. Put `libonnxruntime.so` next to the process or in the image (`./scripts/fetch-onnxruntime.sh`).
4. Mount models read-only. Embedder-only: **1–2 GiB**. Embedder + reranker: **4 GiB**. Kind e2e’s `512Mi` is fake-embedder only.

Compose (models directory next to the compose file):

```bash
docker compose -f deploy/compose.onnx.yml up --build
```

ECS/EKS sketch:

| Knob | Value |
| --- | --- |
| Command | `klimatsearch` (no `--demo-fixture`) |
| `KLIMAT_EMBEDDER` | `onnx` |
| `KLIMAT_RERANKER` | `onnx` (or `none`) |
| `KLIMAT_RERANKER_MODEL` | `bge-reranker-v2-m3` |
| `KLIMAT_MODELS` | `/models` |
| `KLIMAT_EMBEDDING_MODEL` | `f2llm-v2-80m` |
| `ONNXRUNTIME_LIB` | `/usr/local/lib/libonnxruntime.so` |
| Volume | EFS / EBS / image layer with `models/f2llm-v2-80m/` and optionally `models/bge-reranker-v2-m3/` |
| Memory | 2 GiB embedder; 4 GiB with reranker |
| Search | clients pass `vector=true` and `rerank=true` |

PVC for `data/klimat.db` if you do not want to re-embed ~230 rows on every cold start.

Production image (self-hosted, no Unkey/MPP):

```bash
docker build --build-arg GO_TAGS="fts5 tokenizers" -t klimatsearch:onnx .
```

The default Dockerfile runtime image is Debian slim **without** libonnxruntime or weights. Copy `libonnxruntime.so` and mount `models/` yourself, or add a private image layer. Do not commit weights.

## Verify

```bash
./scripts/check-models.sh
curl -sS http://127.0.0.1:8080/healthz
# first ONNX ingest logs embedding work; then:
curl -sS 'http://127.0.0.1:8080/api/search?q=betong&lang=sv&vector=true' | head
```

If start fails, the error names the missing file or `onnxruntime init` — set `ONNXRUNTIME_LIB` to the `.so` / `.dylib`.
