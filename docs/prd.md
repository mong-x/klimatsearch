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

1. Boverket Ingester: `GET {BOVERKET_API_BASE}/api/Klimat/v2/GetAllResources/latest/{sv|en}/json` on default base `https://api.boverket.se/klimatdatabas`. Optional `BOVERKET_SUBSCRIPTION_KEY` as `Ocp-Apim-Subscription-Key`.
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
| GET | `/api/search?q=&vector=&rerank=&lang=&databases=` | lang default `sv`; `databases` is Catalog IDs (`boverket,br25`); empty q or unknown lang → 400. Each hit is the full Resource plus `score`, `match_source`, `details` (`/api/resources/{catalog}:{id}`), and `origin` (Boverket product sheet) when known |
| GET | `/api/resources?lang=&databases=` | list |
| GET | `/api/resources/{id}?lang=` | 404 if missing; `{id}` may be prefixed `boverket:6000000000` or bare Resource ID when only one Catalog matches. JSON includes `origin` for Boverket sheets |
| GET | `/api/resources/{id}/origin` | 302 to the Catalog's official page (`klimatdatabasen.boverket.se/detaljer/{code}/{id}`); 404 if that Catalog has no public sheet |
| GET | `/api/resources/compare?a=&b=&unit=` | unit optional; 404 if a Resource ID is missing; 400 if explicit unit cannot apply |
| POST | `/admin/ingest/file?catalog=` | multipart file for file-only Catalogs (BR25); Guard + admin |

JSON includes Attribution `source: "Boverket Klimatdatabas"` and Catalog ID `catalog`.

## MCP tools

1. `search_climate_data` — query, optional lang. Each hit is the full Resource plus `details`. Vector + rerank when those backends are enabled; if reranker is `none`, vector only.
2. `get_resource_details` — id (bare Resource ID or `catalog:id`; same as the hit `details` path)
3. `compare_resources` — id_a, id_b, optional unit — A1–A3 in a shared unit using Conversion

## Non-goals (v1)

- Hosting ONNX weights in git
- Write-back to Boverket
- Alternative routers or CGO-free SQLite
- Implementing BR25 file mapping until a sample workbook exists (Ingester + admin route ship; parser can reject unknown Catalogs)

## 5. Monetization & authentication (dual-lane Guard)

Same binary serves AI agents and human/SaaS developers.

### 5.1 Agent lane (MPP)

Agents pay in the same HTTP request via the Machine Payments Protocol. klimatsearch verifies with official `github.com/tempoxyz/mpp-go` (`Charge` / Tempo `MethodFromConfig`). Unpaid requests get HTTP 402 and `WWW-Authenticate: Payment`. No in-process payment math beyond the SDK charge.

### 5.2 Developer lane (Unkey + Stripe)

`Authorization: Bearer <KEY>` verified with `github.com/unkeyed/sdks/api/go/v2` `Keys.VerifyKey`. Unkey meters; Stripe bills. Do not use the archived `github.com/unkeyed/unkey-go` module.

### 5.3 Guard order

On `/api/search`, resource routes, MCP, and admin ingest:

1. `Authorization: Payment` → MPP Charge; invalid or missing credential → 402 challenge.
2. Else Bearer token → Unkey; invalid → 401.
3. Else if MPP is configured → 402 Payment challenge. Else 401.

`GET /healthz` is public. If Unkey and MPP credentials are both unset, the Guard allows all traffic (local/kind). If either is configured, fail-closed.

## 6. Multi-Catalog extensibility

Resource identity is `(catalog_id, resource_id)`. sqlite-vec document id is `{catalog_id}:{resource_id}`. REST/MCP accept optional Catalog filters (`databases=`).

Boverket Ingester uses the OpenAPI v2 JSON API. File-only Catalogs (BR25) use `POST /admin/ingest/file?catalog=br25`. Weekly poll stays Boverket-only until a Catalog has an HTTP Ingester.
