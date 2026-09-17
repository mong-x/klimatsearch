<div align="center">

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:0A0B0D,45:1F6F4A,100:82AAFF&height=200&section=header&text=klimatsearch&fontColor=E6E8EB&fontSize=64&fontAlignY=38&animation=fadeIn&desc=Hybrid%20search%20%2B%20MCP%20for%20Boverket%20Klimatdatabas&descSize=16&descAlignY=62" width="100%" alt="klimatsearch — hybrid search and MCP for Boverket Klimatdatabas" />

[![CI](https://img.shields.io/github/actions/workflow/status/mong-x/klimatsearch/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=E6E8EB&label=CI&labelColor=0A0B0D&color=1F6F4A)](https://github.com/mong-x/klimatsearch/actions/workflows/ci.yml)
[![kind e2e](https://img.shields.io/github/actions/workflow/status/mong-x/klimatsearch/e2e-kind.yml?branch=main&style=for-the-badge&logo=kubernetes&logoColor=E6E8EB&label=kind%20e2e&labelColor=0A0B0D&color=82AAFF)](https://github.com/mong-x/klimatsearch/actions/workflows/e2e-kind.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/mong-x/klimatsearch?style=for-the-badge&logo=go&logoColor=E6E8EB&label=Go&labelColor=0A0B0D&color=82AAFF)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache_2.0-1F6F4A?style=for-the-badge&labelColor=0A0B0D)](./LICENSE)
[![MCP](https://img.shields.io/badge/MCP-streamable%20HTTP-82AAFF?style=for-the-badge&labelColor=0A0B0D)](https://modelcontextprotocol.io)
[![Pages](https://img.shields.io/badge/site-mong--x.github.io-1F6F4A?style=for-the-badge&logo=githubpages&logoColor=E6E8EB&labelColor=0A0B0D)](https://mong-x.github.io/klimatsearch/)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-Swagger_UI-82AAFF?style=for-the-badge&labelColor=0A0B0D)](https://mong-x.github.io/klimatsearch/swagger/)

<sub>SQLite FTS5 · sqlite-vec · RRF k=60 · F2LLM-v2-80M · Unkey · MPP 402</sub>

**[Live site →](https://mong-x.github.io/klimatsearch/)** · **[Swagger UI →](https://mong-x.github.io/klimatsearch/swagger/)** — REST spec (`GET /openapi.yaml`, `GET /docs` on a running process).

**Get running in one line:** `make run` — then `GET http://127.0.0.1:8080/api/search?q=betong&lang=sv`

</div>

<p align="center">
  <img src="site/assets/og.jpg" alt="klimatsearch: hybrid search and MCP over Boverket Klimatdatabas, shown as a terminal GET /api/search?q=betong" width="920" />
</p>

## Introduction

**klimatsearch is a distributed search engine and [MCP](https://modelcontextprotocol.io) server for [Boverket Klimatdatabas](https://www.boverket.se/sv/klimatdeklaration/klimatdatabas/)** — the Swedish National Board of Housing, Building and Planning's generic construction climate database (~230 bilingual resources, typical A1–A3).

It is a single statically compiled Go binary: stdlib `net/http.ServeMux`, SQLite with FTS5 + [sqlite-vec](https://github.com/asg017/sqlite-vec), optional ONNX embeddings, REST, and MCP. Agents and BIM tools search by Swedish or English name, compare A1–A3 in a shared unit, and cite **Boverket Klimatdatabas**.

This project is **not affiliated with Boverket**. Klimatdatabas content remains Boverket's; you must cite Boverket when you use it.

**Who it's for:** LCA / klimatdeklaration workflows, MCP-capable coding agents, and SaaS tools that need generic Swedish construction Resources without scraping Excel by hand.

- Hybrid retrieval — FTS5 BM25 always; sqlite-vec KNN when vectors exist; fused with Reciprocal Rank Fusion (`k=60`)
- Catalog identity — `(catalog_id, resource_id)` from day one (`boverket`, later `br25`)
- Dual-lane Guard — agents pay with MPP (`Authorization: Payment` → HTTP 402); developers use Unkey Bearer
- Fake embedder default — `make run` and CI need no model weights

Software is ready for a laptop demo. Accounts, ONNX weights, and production hosting are yours — **[docs/LAUNCH.md](docs/LAUNCH.md)**.

## Table of contents

- [Quick start](#quick-start)
- [Requirements](#requirements)
- [REST](#rest)
- [MCP](#mcp)
- [Search](#search)
- [Catalogs and ingest](#catalogs-and-ingest)
- [Guard](#guard)
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
- Build tag **`fts5`** — mattn/go-sqlite3 compiles FTS5 only with that tag. `make test` / `make build` pass it. Bare `go test ./...` fails with `no such module: fts5` ([ADR-0001](docs/adr/0001-go-stdlib-mux-and-cgo-sqlite.md)).

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

JSON includes Attribution `"source": "Boverket Klimatdatabas"` and `"catalog"` (Catalog ID). Each **search hit is the full Resource** (names, descriptions, applicability, synonyms, conversions, A1A3, category, version) plus `score`, `match_source` (`fts` \| `vector` \| `both` \| `rerank`), `details` (`/api/resources/{catalog}:{id}`), and — for Boverket rows — `origin` (the official product sheet on [klimatdatabasen.boverket.se](https://klimatdatabasen.boverket.se)). Boverket's OpenAPI has no per-id GET; `origin` is `https://klimatdatabasen.boverket.se/detaljer/{category_code}/{resource_id}`. `GET {details}/origin` **302**s there. Follow `details` (or MCP `get_resource_details`) for klimatsearch's copy. Do not confuse `match_source` with the citation `source`.

Admin file ingest is multipart field `file`, Guard-protected. Optional `KLIMAT_ADMIN_TOKEN` as `X-Admin-Token`. Unknown Catalog → 400.

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

Point Claude / Cursor / Codex at the streamable endpoint. Paid MCP routes go through the same Guard as REST.

## Search

Hybrid retrieval ([ADR-0006](docs/adr/0006-rrf-k60-hybrid-hits.md)):

1. **FTS5** BM25 always runs on names, descriptions, applicability, synonyms.
2. **sqlite-vec** KNN runs when the store has vectors and `vector=true`.
3. **RRF** (`k=60`, equal weights) fuses the two ranked lists.
4. Optional ONNX reranker is a compile stub (`--reranker=none` until embeddings are trusted).

Vector off → FTS only. A stored `meta.embedding_dim` mismatch disables vector search rather than mixing dimensions.

## Catalogs and ingest

Identity is `(catalog_id, resource_id)` ([ADR-0007](docs/adr/0007-catalog-identity.md)). sqlite-vec document id is `{catalog_id}:{resource_id}`.

On start (default) and every `--ingest-interval` (default `168h`):

1. GET `{BOVERKET_API_BASE}/api/Klimat/v2/GetAllResources/latest/{sv,en}/json` (default base `https://api.boverket.se/klimatdatabas`)
2. Merge cultures by ResourceId
3. Fall back to the public Excel files on HTTP error, parse error, or an empty list

Optional `BOVERKET_SUBSCRIPTION_KEY` as `Ocp-Apim-Subscription-Key`. Live v2 JSON currently returns 200 without a key.

File-only Catalogs (Denmark BR25) use `POST /admin/ingest/file?catalog=br25`. Mapping returns not-implemented until a sample workbook exists ([LAUNCH.md](docs/LAUNCH.md) §3).

Changed rows are detected with a canonical **ContentHash** of names, descriptions, applicability, synonyms, A1A3, Declared unit, Conversions, Category, Category Code, and DatasetVersion — not Resource ID or Catalog ID — and re-embedded. If any row was upserted, klimatsearch POSTs `catalog.changed` to `KLIMAT_WEBHOOK_URL` (optional HMAC). Unchanged ingest does not fire ([ADR-0011](docs/adr/0011-webhook-on-content-change.md)).

## Guard

Paid routes (`/api/*`, `/mcp`, `/admin/*`) go through one Guard ([ADR-0008](docs/adr/0008-dual-lane-guard.md)). `GET /healthz` is public.

1. `Authorization: Payment …` → [mpp-go](https://github.com/tempoxyz/mpp-go) Tempo charge. Missing credential → **HTTP 402** + `WWW-Authenticate: Payment`.
2. Else `Authorization: Bearer …` → Unkey `Keys.VerifyKey` (`github.com/unkeyed/sdks/api/go/v2`). Invalid → 401.
3. Else if MPP is configured → 402 challenge. Else 401.

When `UNKEY_ROOT_KEY` and `MPP_SECRET_KEY` are both unset, the Guard **allows all traffic** so `make run` and kind e2e keep working. The moment either secret is set, it is fail-closed.

Copy `.env.example` to a gitignored `.env`. Existing shell exports win over `.env`.

| Env | Role |
| --- | --- |
| `UNKEY_ROOT_KEY` | Server-only root key (`unkey_…`) to call VerifyKey. **Not** a customer `sk_…` key. |
| `MPP_SECRET_KEY` | HMAC for challenge IDs (`openssl rand -hex 32`) |
| `MPP_RECIPIENT` | Tempo `0x…` payee (required with the secret). Default RPC is Tempo testnet `https://rpc.moderato.tempo.xyz`. |

Do not paste secrets into issues or chat. Walkthrough: [LAUNCH.md](docs/LAUNCH.md) §2.

## Embeddings

Default production model is [codefuse-ai/F2LLM-v2-80M](https://huggingface.co/codefuse-ai/F2LLM-v2-80M) (hidden size 320, last-token/EOS pool, L2). Weights are **not** in git.

```bash
brew install onnxruntime          # macOS; or set ONNXRUNTIME_LIB
./scripts/fetch-libtokenizers.sh  # CGO lib for HuggingFace tokenizer.json
make models                       # Hugging Face download (uv or huggingface-cli)
python scripts/export-f2llm-onnx.py
make build
KLIMAT_EMBEDDER=onnx KLIMAT_LISTEN=:8081 ./bin/klimatsearch
```

`auto` uses ONNX if `models/<embedding-model>/model.onnx` exists, otherwise fake. `--embedder` / `KLIMAT_EMBEDDER` always wins. CI uses Fake and does not need ONNX Runtime.

F2LLM-v2 is **asymmetric**: documents get a labeled bilingual `EmbeddingText` with no Instruct prefix; queries use `Instruct:…\nQuery:` ([ADR-0010](docs/adr/0010-f2llm-query-prefix.md)). Fake hashes the raw Query so tests stay model-free.

To swap models, point `--models` / `--embedding-model` at a new directory. Re-embed after a dimension change.

## Configuration

Flags win over env over defaults. A gitignored `.env` next to `go.mod` is loaded first.

| Flag / env | Default | Meaning |
| --- | --- | --- |
| `--listen` / `KLIMAT_LISTEN` | `:8080` | HTTP bind |
| `--db` / `KLIMAT_DB` | `./data/klimat.db` | SQLite path |
| `--embedder` / `KLIMAT_EMBEDDER` | `auto` | `auto` \| `fake` \| `onnx` |
| `--reranker` / `KLIMAT_RERANKER` | `none` | `none` \| `fake` \| `onnx` |
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
```

CI builds a linux binary artifact `klimatsearch-linux-amd64` and runs the kind e2e workflow (`cluster_name: klimatsearch-e2e`). Kind uses the fake embedder and `--demo-fixture`. ONNX in a cluster needs more memory than the e2e `512Mi` limit (1–2 Gi is a starting point).

## Documentation

Full index: **[docs/README.md](docs/README.md)**.

| Doc | What's in it |
| --- | --- |
| [docs/LAUNCH.md](docs/LAUNCH.md) | ONNX, Unkey, MPP, BR25, production k8s — human-only steps |
| [docs/prd.md](docs/prd.md) | Product spec |
| [CONTEXT.md](CONTEXT.md) | Glossary |
| [docs/adr/](docs/adr/) | Architecture Decision Records |
| [site/](site/) | GitHub Pages source |

## License

[Apache-2.0](./LICENSE). Klimatdatabas content remains Boverket's; cite **Boverket Klimatdatabas** on every public surface.

README header by [capsule-render](https://github.com/kyechan99/capsule-render).

<div align="center">
<img src="https://capsule-render.vercel.app/api?type=waving&color=0:82AAFF,55:1F6F4A,100:0A0B0D&height=120&section=footer" width="100%" alt="" />
<sub>Search Klimatdatabas. Cite Boverket.</sub>
</div>
