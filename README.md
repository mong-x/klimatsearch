<div align="center">

<img src="site/assets/hero.jpg" alt="Particle board, ready-mix concrete, and glass wool — generic construction resources from Boverket Klimatdatabas" width="100%" />

[![CI](https://img.shields.io/github/actions/workflow/status/mong-x/klimatsearch/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=E6E8EB&label=CI&labelColor=0A0B0D&color=1F6F4A)](https://github.com/mong-x/klimatsearch/actions/workflows/ci.yml)
[![kind e2e](https://img.shields.io/github/actions/workflow/status/mong-x/klimatsearch/e2e-kind.yml?branch=main&style=for-the-badge&logo=kubernetes&logoColor=E6E8EB&label=kind%20e2e&labelColor=0A0B0D&color=82AAFF)](https://github.com/mong-x/klimatsearch/actions/workflows/e2e-kind.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/mong-x/klimatsearch?style=for-the-badge&logo=go&logoColor=E6E8EB&label=Go&labelColor=0A0B0D&color=82AAFF)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache_2.0-1F6F4A?style=for-the-badge&labelColor=0A0B0D)](./LICENSE)
[![MCP](https://img.shields.io/badge/MCP-streamable%20HTTP-82AAFF?style=for-the-badge&labelColor=0A0B0D)](https://modelcontextprotocol.io)
[![Pages](https://img.shields.io/badge/site-mong--x.github.io-1F6F4A?style=for-the-badge&logo=githubpages&logoColor=E6E8EB&labelColor=0A0B0D)](https://mong-x.github.io/klimatsearch/)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-Swagger_UI-82AAFF?style=for-the-badge&labelColor=0A0B0D)](https://mong-x.github.io/klimatsearch/swagger/)

<sub>SQLite FTS5 · sqlite-vec · RRF k=60 · F2LLM-v2-80M · self-hosted ungated</sub>

**[Live site →](https://mong-x.github.io/klimatsearch/)** · **[Swagger UI →](https://mong-x.github.io/klimatsearch/swagger/)** · **[llms.txt →](llms.txt)**

**Get running in one line:** `make run` — then `GET http://127.0.0.1:8080/api/search?q=betong&lang=sv`

</div>

<p align="center">
  <img src="site/assets/og.jpg" alt="Real klimatsearch response for q=spånskiva: Resource 6000000000 with typical A1–A3 0.39, A4 0.0629, A5.1 0.055" width="920" />
</p>

## Introduction

**klimatsearch is a distributed search engine and [MCP](https://modelcontextprotocol.io) server for [Boverket Klimatdatabas](https://www.boverket.se/sv/klimatdeklaration/klimatdatabas/)** — the Swedish National Board of Housing, Building and Planning's generic construction climate database (~230 bilingual resources, typical A1–A3).

It is a single statically compiled Go binary: stdlib `net/http.ServeMux`, SQLite with FTS5 + [sqlite-vec](https://github.com/asg017/sqlite-vec), optional ONNX embeddings, REST, and MCP. Coding agents and klimatdeklaration software search by Swedish or English name, compare A1–A3 in a shared unit, and cite **Boverket Klimatdatabas**.

This project is **not affiliated with Boverket**. Klimatdatabas content remains Boverket's; you must cite Boverket when you use it.

**Who it's for:** LCA / klimatdeklaration workflows, MCP-capable coding agents, and SaaS tools that need generic Swedish construction Resources without scraping Excel by hand.

- Hybrid retrieval — FTS5 BM25 always; sqlite-vec KNN when vectors exist; fused with Reciprocal Rank Fusion (`k=60`)
- Catalog identity — `(catalog_id, resource_id)` from day one (`boverket`, later `br25`)
- Fake embedder default — `make run` and CI need no model weights
- Self-hosted build has **no Unkey and no MPP**; ONNX reranker is the same binary (`KLIMAT_RERANKER=onnx`)
- Agents: [llms.txt](llms.txt) + [docs/AGENT-SELFHOST.md](docs/AGENT-SELFHOST.md) — MCP, cheap path, HPA

Software is ready for a laptop demo. Accounts, ONNX weights, and production hosting are yours — **[docs/LAUNCH.md](docs/LAUNCH.md)**.

## Table of contents

- [Quick start](#quick-start)
- [Requirements](#requirements)
- [REST](#rest)
- [MCP](#mcp)
- [Agents](#agents)
- [Search](#search)
- [Retrieval quality](#retrieval-quality)
- [Catalogs and ingest](#catalogs-and-ingest)
- [Self-hosted vs hosted](#self-hosted-vs-hosted)
- [Embeddings](#embeddings)
- [Configuration](#configuration)
- [Docker and kind](#docker-and-kind)
- [Documentation](#documentation)
- [License](#license)

## Quick start

```bash
git clone https://github.com/mong-x/klimatsearch.git
cd klimatsearch
make run
# GET http://127.0.0.1:8080/healthz
# GET http://127.0.0.1:8080/api/search?q=betong&lang=sv
```

`make run` builds with `-tags fts5`, starts the fake Embedder, and loads `testdata/fixtures/resources.json` (`--demo-fixture`) so a laptop demo works without the live Boverket API.

If port 8080 is taken (common with a local Node process):

```bash
KLIMAT_LISTEN=:8081 make run
# then curl http://127.0.0.1:8081/api/search?q=betong&lang=sv
```

Live ingest (no fixture) from the official JSON API:

```bash
make build
./bin/klimatsearch --embedder=fake --listen=:8081
```

## Requirements

- **Go 1.27** and **CGO** (`CGO_ENABLED=1`)
- **gcc** (Linux: `libsqlite3-dev`)
- Build tag **`fts5`** — mattn/go-sqlite3 compiles FTS5 only with that tag. `make test` / `make build` pass it. Bare `go test ./...` fails with `no such module: fts5`.

Optional for production embeddings: [ONNX Runtime](https://onnxruntime.ai) (`brew install onnxruntime` or `ONNXRUNTIME_LIB`), Hugging Face tokenizer C library (`./scripts/fetch-libtokenizers.sh`), and F2LLM-v2-80M weights (`make models`).

## REST

| Method | Path |
| --- | --- |
| GET | `/healthz` |
| GET | `/openapi.yaml` |
| GET | `/docs` |
| GET | `/api/search?q=&vector=true\|false&rerank=true\|false&lang=sv\|en&databases=` |
| GET | `/api/resources?lang=&databases=` |
| GET | `/api/resources/{id}?lang=` |
| GET | `/api/resources/{id}/origin` |
| GET | `/api/resources/compare?a={id}&b={id}&unit=` |
| POST | `/admin/ingest/file?catalog=` |

`vector` and `rerank` default to **false**. Compare `unit` is optional; if you pass a unit that cannot apply to both Resources, the API returns **400**. `databases` is a comma-separated list of Catalog IDs (`boverket`, `br25`); empty means all Catalogs.

Resource `{id}` may be prefixed (`boverket:6000000000`) or a bare Resource ID when only one Catalog matches (**409** if ambiguous).

JSON includes Attribution `"source": "Boverket Klimatdatabas"` and `"catalog"` (Catalog ID). Each **search hit is the full Resource** (names, descriptions, applicability, synonyms, conversions, typical A1–A3, **conservative A1–A3, A4, A5.1**, waste/conservative factors, biogenic carbon, service life, BK04, A4 transport legs, category, version) plus `score`, `match_source` (`fts` \| `vector` \| `both` \| `rerank`), `details` (`/api/resources/{catalog}:{id}`), and — for Boverket rows — `origin` (the official product sheet on [klimatdatabasen.boverket.se](https://klimatdatabasen.boverket.se)). Energy carriers omit A4/A5.1. Compare still uses **typical** A1–A3. Boverket's OpenAPI has no per-id GET; `origin` is `https://klimatdatabasen.boverket.se/detaljer/{category_code}/{resource_id}`. `GET {details}/origin` **302**s there. Follow `details` (or MCP `get_resource_details`) for klimatsearch's copy. Do not confuse `match_source` with the citation `source`.

Admin file ingest is multipart field `file`. Optional `KLIMAT_ADMIN_TOKEN` as `X-Admin-Token`. Unknown Catalog → 400. Self-hosted does not require Unkey or MPP.

```bash
curl -sS 'http://127.0.0.1:8081/api/search?q=spånskiva&lang=sv'
curl -sS 'http://127.0.0.1:8081/api/resources/boverket:6000000000?lang=en'
curl -sS 'http://127.0.0.1:8081/api/resources/compare?a=6000000000&b=6000000001&unit=kg'
```

## MCP

- Streamable HTTP: `POST`/`GET` `/mcp`
- Legacy SSE: `GET /mcp/sse`, messages `POST /mcp/messages?sessionid=` (also accepted on `/mcp/sse`)

| Tool | Arguments |
| --- | --- |
| `search_climate_data` | query, optional `lang`, optional `databases` array. Hits are full Resources plus `details` |
| `get_resource_details` | Resource ID (prefixed or bare); same as `GET {hit.details}` |
| `compare_resources` | `id_a`, `id_b`, optional `unit` |

Point Claude / Cursor / Codex at the streamable endpoint. The hosted build gates MCP the same way as REST; self-hosted does not.

## Agents

Load **[llms.txt](llms.txt)** (index) then **[docs/AGENT-SELFHOST.md](docs/AGENT-SELFHOST.md)** (playbook). After Pages deploy: `https://mong-x.github.io/klimatsearch/llms.txt`.

MCP client (process already listening):

```json
{
  "mcpServers": {
    "klimatsearch": {
      "url": "http://127.0.0.1:8080/mcp"
    }
  }
}
```

Spend the cheapest path that can hit the Resource:

| Call | When | Cost (this catalog, CPU) |
| --- | --- | --- |
| FTS (REST default) | Exact name in the query language | 0.1 ms |
| `vector=true` | Paraphrase, translation, missing FTS tokens | ~112 ms |
| `rerank=true` | Sibling grades after hybrid | **~7.3 s p50** (BGE-m3 INT8, 20 hits) |

REST defaults **vector and rerank off**. MCP uses process config: `KLIMAT_RERANKER=onnx` means every MCP search reranks. Cache `(q, lang)` in the session. Cite **Boverket Klimatdatabas**. Scale-out is replica-local SQLite + [HPA](deploy/k8s/hpa.onnx.yaml), not one shared WAL.

## Search

Hybrid retrieval:

1. **FTS5** BM25 always runs on names, descriptions, applicability, synonyms.
2. **sqlite-vec** KNN runs when the store has vectors and `vector=true`.
3. **RRF** (`k=60`, equal weights) fuses the two ranked lists.
4. Optional ONNX reranker (`KLIMAT_RERANKER=onnx`, `rerank=true`) after F2LLM is trusted. Measured production pick is **BGE-m3 INT8** (`KLIMAT_ONNX_QUANT=auto`). zerank-1-small fp32 matches BGE fp32 Hit@1 at ~4× the latency; **do not use zerank INT8** (quality collapses). Do not load zerank-2.

Vector off → FTS only. A stored `meta.embedding_dim` mismatch disables vector search rather than mixing dimensions.

## Retrieval quality

Measured on Boverket Klimatdatabas **02.07.000** (230 resources) with 44 labeled queries in [`testdata/golden-queries.json`](testdata/golden-queries.json): sibling grades (glasull, Fabriksbetong C-class, gips, …), exact-name controls, and cross-lingual paraphrases. The labeled Resource must rank first; a listed sibling must not outrank it.

`make eval-golden` — F2LLM-ingested `data/klimat.db`, not the Fake embedder.

**Ship this:** hybrid retrieval (F2LLM) + **BGE-m3 INT8** rerank. That is the only lane with 44/44 Hit@1.

| Stage | Use | Hit@1 | Hit@3 | MRR | Inversions | Miss@20 | p50 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| FTS only | Always on (cheap lexical) | 0.750 | 0.750 | 0.750 | 0.000 | 0.250 | 0.1 ms |
| Hybrid (FTS + F2LLM) | **Default retrieval** (`vector=true`) | 0.955 | 0.977 | 0.972 | 0.045 | **0.000** | 112 ms |
| Hybrid + BGE-m3 fp32 | Skip — INT8 is better and faster | 0.955 | **1.000** | 0.977 | 0.045 | **0.000** | 11.1 s |
| Hybrid + **BGE-m3 INT8** | **Use this reranker** (`KLIMAT_RERANKER=onnx`, `KLIMAT_ONNX_QUANT=auto`) | **1.000** | **1.000** | **1.000** | **0.000** | **0.000** | **7.3 s** |
| Hybrid + zerank-1-small fp32 | Skip — same Hit@1 as BGE fp32, ~6× slower | 0.955 | **1.000** | 0.977 | 0.045 | **0.000** | 42.3 s |
| Hybrid + zerank-1-small INT8 | **Never** — ranking is broken | 0.568 | 0.864 | 0.731 | 0.273 | **0.000** | 14.9 s |

INT8 vs fp32 top-1 agreement: BGE **0.955** (INT8 *fixes* the two remaining fp32 inversions). zerank **0.568** — INT8 is not a drop-in for that graph.

**What the columns mean**

| Metric | What it answers |
| --- | --- |
| **Hit@1** | Did the labeled Resource come back as rank 1? This is the klimatdeklaration case: the agent or tool should pick the right product, not a sibling. |
| **Hit@3** | Is it in the top three? Useful when a human still glances at the list. |
| **MRR** | Mean reciprocal rank (`1/rank`, 0 if missing). Penalises “it was 7th” more honestly than Hit@3. |
| **Inversions** | A `near_miss` sibling ranked **above** the labeled Resource (C25/30 vs klimatförbättrad C25/30). Rank-1 can still be wrong even when the right row is in the list. |
| **Miss@20** | Labeled id not in the 20-hit window at all. Hybrid/rerank cannot recover a document that retrieval never saw. |
| **p50** | Median time to score one query at that stage (20 candidates for rerank). |

**How to read this snapshot.** Keyword search is fast and brittle: 75% Hit@1, and **11/44 queries return no FTS hits**. F2LLM hybrid recovers every miss (two sibling inversions remain: klimatförbättrad C-class vs ordinary). **BGE-m3 INT8 is the only reranker to ship**: 44/44 Hit@1, no inversions, faster than BGE fp32. Skip BGE fp32 and zerank fp32. **Never ship zerank INT8** (Hit@1 56.8%). `KLIMAT_ONNX_QUANT=auto` already prefers BGE `model.int8.onnx`.

Reproduce:

```bash
# data/klimat.db must have been ingested with KLIMAT_EMBEDDER=onnx
make eval-golden
```

Snapshot: 2026-09-18, macOS, ONNX Runtime CPU, n=44, F2LLM-v2-80M + BGE-m3 and zerank-1-small (fp32 and INT8).

## Catalogs and ingest

Identity is `(catalog_id, resource_id)`. sqlite-vec document id is `{catalog_id}:{resource_id}`.

On start (default) and every `--ingest-interval` (default `168h`):

1. GET `{BOVERKET_API_BASE}/api/Klimat/v2/GetAllResources/latest/{sv,en}/json` (default base `https://api.boverket.se/klimatdatabas`)
2. Merge cultures by ResourceId
3. Fall back to the public Excel files on HTTP error, parse error, or an empty list

Optional `BOVERKET_SUBSCRIPTION_KEY` as `Ocp-Apim-Subscription-Key`. Live v2 JSON currently returns 200 without a key.

File-only Catalogs (Denmark BR25) use `POST /admin/ingest/file?catalog=br25`. Mapping returns not-implemented until a sample workbook exists ([LAUNCH.md](docs/LAUNCH.md) §3).

Changed rows are detected with a canonical **ContentHash** of names, descriptions, applicability, synonyms, A1A3, Declared unit, Conversions, Category, Category Code, and DatasetVersion — not Resource ID or Catalog ID — and re-embedded. If any row was upserted, klimatsearch POSTs `catalog.changed` to `KLIMAT_WEBHOOK_URL` (optional HMAC). Unchanged ingest does not fire.

## Self-hosted vs hosted

`make build` / `make run` / kind e2e compile **without** Unkey or mpp-go. Paid env vars are ignored. That is the binary you self-host.

Self-host with embeddings **and** reranker (same OSS binary):

```bash
export KLIMAT_EMBEDDER=onnx
export KLIMAT_RERANKER=onnx          # BGE-m3; fail-closed if the ONNX file is missing
export KLIMAT_ONNX_QUANT=auto
./bin/klimatsearch --listen=:8080
# REST:  /api/search?q=…&vector=true&rerank=true
# MCP:   reranks automatically when KLIMAT_RERANKER=onnx
```

Layout, compose, and k8s: **[docs/SELFHOST.md](docs/SELFHOST.md)**. Agent steps and HPA: **[docs/AGENT-SELFHOST.md](docs/AGENT-SELFHOST.md)**.

The process we run is `make build-hosted` (`-tags hosted`): MPP `Authorization: Payment` (HTTP 402) and Unkey Bearer. See [LAUNCH.md](docs/LAUNCH.md) §2. Do not paste those secrets into issues or chat.

Copy `.env.example` to a gitignored `.env`. Existing shell exports win over `.env`.

## Embeddings

Default production model is [codefuse-ai/F2LLM-v2-80M](https://huggingface.co/codefuse-ai/F2LLM-v2-80M) (hidden size 320, last-token/EOS pool, L2). Weights are **not** in git. Production reranker is [BAAI/bge-reranker-v2-m3](https://huggingface.co/BAAI/bge-reranker-v2-m3) INT8 (`KLIMAT_RERANKER=onnx`, `KLIMAT_ONNX_QUANT=auto`, search `rerank=true`) — 44/44 Hit@1 on the golden set. [zerank-1-small](https://huggingface.co/zeroentropy/zerank-1-small) fp32 is optional and slower with no Hit@1 gain; **do not use zerank INT8**. Do not load zerank-2. Full layout: **[docs/SELFHOST.md](docs/SELFHOST.md)**.

```bash
brew install onnxruntime          # macOS; or set ONNXRUNTIME_LIB
./scripts/fetch-libtokenizers.sh  # CGO lib for HuggingFace tokenizer.json
make models                       # Hugging Face download (uv or huggingface-cli)
python scripts/export-f2llm-onnx.py
python scripts/export-bge-reranker-onnx.py
python scripts/quantize-onnx.py models/f2llm-v2-80m
python scripts/quantize-onnx.py models/bge-reranker-v2-m3
make build
KLIMAT_EMBEDDER=onnx KLIMAT_RERANKER=onnx KLIMAT_LISTEN=:8081 ./bin/klimatsearch
```

`auto` uses ONNX if `models/<embedding-model>/model.onnx` exists, otherwise fake. `--embedder` / `KLIMAT_EMBEDDER` always wins. CI uses Fake and does not need ONNX Runtime.

F2LLM-v2 is **asymmetric**: documents get a labeled bilingual `EmbeddingText` with no Instruct prefix; queries use `Instruct:…\nQuery:`. Fake hashes the raw Query so tests stay model-free.

To swap models, point `--models` / `--embedding-model` at a new directory. Re-embed after a dimension change.

## Configuration

Flags win over env over defaults. A gitignored `.env` next to `go.mod` is loaded first.

| Flag / env | Default | Meaning |
| --- | --- | --- |
| `--listen` / `KLIMAT_LISTEN` | `:8080` | HTTP bind |
| `--db` / `KLIMAT_DB` | `./data/klimat.db` | SQLite path |
| `--embedder` / `KLIMAT_EMBEDDER` | `auto` | `auto` \| `fake` \| `onnx` |
| `--reranker` / `KLIMAT_RERANKER` | `none` | `none` \| `fake` \| `onnx` |
| `--reranker-model` / `KLIMAT_RERANKER_MODEL` | `bge-reranker-v2-m3` | also `zerank-1-small` |
| `--onnx-quant` / `KLIMAT_ONNX_QUANT` | `auto` | `auto` \| `int8` \| `fp32` — prefers `model.int8.onnx` |
| `--ingest-on-start` | true | Run ingest once before listen |
| `--ingest-interval` | `168h` | Repeat ingest; `0` disables |
| `--demo-fixture` | false | Load testdata JSON, no network |
| `KLIMAT_WEBHOOK_URL` | empty | Comma-separated URLs; POST `catalog.changed` when ingest upserts |
| `KLIMAT_WEBHOOK_SECRET` | empty | HMAC-SHA256 of the JSON body (`X-Klimat-Signature`) |
| `--boverket-api-base` | `https://api.boverket.se/klimatdatabas` | APIM base |

## Docker and kind

```bash
make docker          # klimatsearch:local
make e2e             # kind cluster + kustomize + HTTP probes
docker compose -f deploy/compose.onnx.yml up --build   # F2LLM + BGE, 4 GiB
```

CI builds a linux binary artifact `klimatsearch-linux-amd64` and runs the kind e2e workflow (`cluster_name: klimatsearch-e2e`). Kind uses the fake embedder, `--demo-fixture`, and `512Mi`. Production ONNX: **1–2 Gi** embedder-only, **4 Gi** with BGE ([`deploy/k8s/deployment.onnx.yaml`](deploy/k8s/deployment.onnx.yaml)), HPA 1–8 ([`deploy/k8s/hpa.onnx.yaml`](deploy/k8s/hpa.onnx.yaml)) with **replica-local SQLite**. `KLIMAT_RERANKER=none docker compose -f deploy/compose.onnx.yml up --build` skips the reranker.

## Documentation

Full index: **[docs/README.md](docs/README.md)**.

| Doc | What's in it |
| --- | --- |
| [docs/LAUNCH.md](docs/LAUNCH.md) | ONNX, Unkey, MPP, webhook, BR25, production k8s — human-only steps |
| [docs/SELFHOST.md](docs/SELFHOST.md) | F2LLM + BGE/zerank ONNX, check-models, Docker/AWS |
| [docs/AGENT-SELFHOST.md](docs/AGENT-SELFHOST.md) | Agent playbook: MCP JSON, cheap path, load, HPA |
| [llms.txt](llms.txt) | llmstxt.org index (`https://mong-x.github.io/klimatsearch/llms.txt`) |
| [Swagger UI](https://mong-x.github.io/klimatsearch/swagger/) | REST OpenAPI |
| [CONTEXT.md](CONTEXT.md) | Glossary |

## License

[Apache-2.0](./LICENSE). Klimatdatabas content remains Boverket's; cite **Boverket Klimatdatabas** on every public surface.

<div align="center">
<sub>Search Klimatdatabas. Cite Boverket.</sub>
</div>
