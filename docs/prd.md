# PRD: klimatsearch

Statically compiled Go search engine and MCP server for Boverket Klimatdatabas (~200 generic construction Resources, Swedish and English).

## Goal

Ingest Klimatdatabas, store it in embedded SQLite (FTS5 + sqlite-vec), optionally embed with a local ONNX model, and expose REST + MCP so tools and humans can search A1–A3 climate data with attribution.

## Constraints

- Router: `net/http.ServeMux` only (Go 1.22+ method matching and `r.PathValue`).
- Database: SQLite via `github.com/mattn/go-sqlite3` (CGO) + `github.com/asg017/sqlite-vec-go-bindings/cgo`. Call `sqlite_vec.Auto()` before opening the DB.
- Migrations: `github.com/pressly/goose/v3` SQL files, embedded with `embed.FS`.
- ML: `github.com/yalue/onnxruntime_go` for production Embedder/Reranker. Tests and default CI must not require ONNX weights or libonnxruntime.
- MCP: official `github.com/modelcontextprotocol/go-sdk`. Streamable HTTP at `/mcp`. Legacy SSE at `/mcp/sse` with messages at `/mcp/messages`.
- Layout: `cmd/` + `internal/`. No `pkg/`.
- License: Apache-2.0. Cite Boverket as the data source.

## Users

Software that calculates climate declarations, and MCP clients that need structured A1–A3 lookup, comparison, and hybrid search.

## Ingest

1. Probe configurable Boverket JSON API (`BOVERKET_API_BASE`, default `https://api.boverket.se`, optional `BOVERKET_SUBSCRIPTION_KEY`).
2. On JSON failure, download the public Swedish and English Excel files and merge by Resource ID.
3. Canonical-JSON hash of key fields. New or changed Resources are embedded (SV+EN text) and upserted.
4. Run on start when `--ingest-on-start` (default true) and on a weekly ticker (`--ingest-interval`, default `168h`).
5. `--demo-fixture` loads `testdata/fixtures` with no network (kind/e2e and local demo).

Live API 404 must not fail boot. Tests never hit the network.

## Search

Hybrid SearchEngine: FTS5 BM25 on the requested language columns, optional sqlite-vec cosine KNN (320-d default, F2LLM-v2-80M) fused with RRF k=60, optional Reranker. Embedding dim is stored in `meta`; mismatch disables vector search and logs clearly.

## HTTP

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/healthz` | `{"status":"ok"}` |
| GET | `/api/search?q=&vector=&rerank=&lang=` | lang default `sv`; empty q or unknown lang → 400 |
| GET | `/api/resources?lang=` | list |
| GET | `/api/resources/{id}?lang=` | 404 if missing |
| GET | `/api/resources/compare?a=&b=&unit=` | unit optional; 404 if a Resource ID is missing; 400 if explicit unit cannot apply |

JSON includes `source: "Boverket Klimatdatabas"`.

## MCP tools

1. `search_climate_data` — query, optional lang. Vector + rerank when those backends are enabled; if reranker is `none`, vector only.
2. `get_resource_details` — id
3. `compare_resources` — id_a, id_b, optional unit — A1–A3 in a shared unit using Conversion

## Non-goals (v1)

- Hosting ONNX weights in git
- Multi-tenant auth
- Write-back to Boverket
- Alternative routers or CGO-free SQLite
