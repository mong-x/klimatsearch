# Agent playbook: self-host klimatsearch

Load this file when you are connecting an agent to klimatsearch, standing up a self-hosted instance, or choosing FTS vs vector vs rerank under load.

**Done when:** the agent can `search_climate_data` (or `GET /api/search`) against a live process, hits include `source: Boverket Klimatdatabas`, and production replicas use replica-local SQLite with HPA — not one shared WAL.

Self-host is `make build` (no Unkey, no MPP). Weights and layout: [SELFHOST.md](SELFHOST.md). Glossary: [CONTEXT.md](../CONTEXT.md).

## 1. Point the agent at MCP

Process must already be listening. Streamable HTTP:

```json
{
  "mcpServers": {
    "klimatsearch": {
      "url": "http://127.0.0.1:8080/mcp"
    }
  }
}
```

Tools: `search_climate_data`, `get_resource_details`, `compare_resources`.

Completion: a search for `spånskiva` / `lang=sv` returns Resource `6000000000` and a `details` path.

REST fallback (same process):

```http
GET /api/search?q=spånskiva&lang=sv
GET /api/search?q=spånskiva&lang=sv&vector=true
GET /api/search?q=fabriksbetong%20C25/30&lang=sv&vector=true&rerank=true
GET /api/resources/boverket:6000000000
GET /api/resources/compare?a=6000000029&b=6000000028&unit=kg
```

Cite **Boverket Klimatdatabas** on every answer. Follow `origin` (or `GET {details}/origin`) for Boverket’s sheet. Do not invent A1–A3.

## 2. Spend the cheap path first

| Call | When | Cost on this catalog (CPU, 02.07.000) |
| --- | --- | --- |
| FTS (default REST) | Exact or near-exact name in the query language | 0.1 ms, no ONNX |
| `vector=true` | Paraphrase, translation, missing FTS tokens | ~112 ms, F2LLM |
| `rerank=true` | Sibling grades after hybrid | **~7.3 s p50** with **BGE-m3 INT8** (44/44 Hit@1). BGE fp32 ~11 s; zerank fp32 ~42 s, same Hit@1 as BGE fp32. **Do not use zerank INT8** (Hit@1 0.57). |

REST defaults **both flags off**. MCP sets vector/rerank from process config (`KLIMAT_RERANKER=onnx` ⇒ every MCP search reranks). For a high-QPS agent fleet, run a pool with `KLIMAT_RERANKER=none` and call REST `vector=true` without rerank unless the top hits are the same family.

Session cache: reuse Hits for the same `(q, lang)` in one conversation. One search, then `get_resource_details` / `compare_resources`. Do not poll.

`KLIMAT_ONNX_QUANT=auto` loads `model.int8.onnx` when present — that is the measured BGE production graph. zerank-1-small is optional fp32 only (~8 GiB, slower, no Hit@1 gain on this catalog). Do not load zerank-2. Do not load zerank INT8.

## 3. Stand up self-host (laptop → compose)

```bash
make build
export KLIMAT_EMBEDDER=onnx
export KLIMAT_RERANKER=onnx
export KLIMAT_ONNX_QUANT=auto
./bin/klimatsearch --listen=:8080
```

Weights: [SELFHOST.md](SELFHOST.md). Compose already enables the reranker:

```bash
docker compose -f deploy/compose.onnx.yml up --build
```

`KLIMAT_RERANKER=none` on that command skips rerank and saves RAM (2 GiB class vs 4 GiB).

Completion: `GET /healthz` is 200; `vector=true&rerank=true` returns `match_source` `rerank` when BGE is loaded.

## 4. Load and autoscale

klimatsearch is **one process + one SQLite**. Scale-out is N independent replicas, each with its own DB file, ingest on start (~230 rows). Do not mount one PVC WAL under several writers.

| Knob | Value |
| --- | --- |
| Replicas | HPA 1–8 on CPU ~70% and memory ~80% — [hpa.onnx.yaml](../deploy/k8s/hpa.onnx.yaml) |
| Memory | 1–2 Gi embedder-only; **4 Gi** F2LLM+BGE; 8 Gi only if that pool runs zerank-1-small |
| Data | emptyDir or per-pod volume at `KLIMAT_DB`; ingest on start. Shared PVC = single replica |
| Models | read-only volume, shared is fine |
| Probes | `GET /healthz` |
| Ingress | timeout ≥ 30 s if any route sends `rerank=true` (BGE ~15 s) |

Apply:

```bash
kubectl apply -f deploy/k8s/deployment.onnx.yaml
kubectl apply -f deploy/k8s/hpa.onnx.yaml
```

For HPA, change the data volume in `deployment.onnx.yaml` from a shared PVC to `emptyDir` (or a unique volume per pod). Keep a PVC only when `replicas: 1` and you want to skip re-embed on restart.

Split pools if MCP load is high:

1. **Hot** — `KLIMAT_RERANKER=none`, 1–2 Gi, FTS + hybrid, many replicas.
2. **Precise** — `KLIMAT_RERANKER=onnx`, 4 Gi, fewer replicas, only sibling queries.

Completion: `kubectl get hpa klimatsearch` shows a target; two pods do not open the same sqlite file.

## 5. Guardrails

- Self-host binary has no Unkey and no MPP. Do not add those env vars to this lane.
- Kind e2e (`512Mi`, fake embedder) is not the production shape.
- `make eval-golden` after ONNX ingest if you change embedder, quant, or reranker.
