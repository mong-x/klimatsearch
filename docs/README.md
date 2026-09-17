# Documentation

Task-oriented index. Start at the [README](../README.md) or the [GitHub Pages site](https://mong-x.github.io/klimatsearch/).

| Doc | What's in it |
| --- | --- |
| [LAUNCH.md](LAUNCH.md) | Human checklist: ONNX weights, Unkey, MPP, BR25, production k8s |
| [prd.md](prd.md) | Product spec (REST, MCP, Guard, Catalogs) |
| [CONTEXT.md](../CONTEXT.md) | Glossary — Catalog, Resource, Guard, Hit, Attribution |
| [AGENTS.md](../AGENTS.md) | Constraints for coding agents (ServeMux, CGO, fts5) |
| [adr/](adr/) | Architecture Decision Records |
| [research/](research/) | Embedding protocol, repo-profile notes |
| [OpenAPI (klimatsearch)](https://mong-x.github.io/klimatsearch/swagger/) | Swagger UI for this process (`internal/api/openapi.yaml`, also `GET /docs`) |
| [reference/boverket-klimatdatabas-openapi.json](reference/boverket-klimatdatabas-openapi.json) | Official Boverket bulk OpenAPI (no per-id GET) |

## ADRs

| ADR | Decision |
| --- | --- |
| [0001](adr/0001-go-stdlib-mux-and-cgo-sqlite.md) | stdlib ServeMux; CGO SQLite + FTS5 |
| [0002](adr/0002-pluggable-embedder-default-f2llm-v2-80m.md) | Fake default; F2LLM-v2-80M production |
| [0003](adr/0003-mcp-streamable-http-plus-legacy-sse.md) | `/mcp` streamable + legacy SSE |
| [0004](adr/0004-kind-for-local-k8s-e2e.md) | kind e2e |
| [0005](adr/0005-boverket-json-api-with-excel-fallback.md) | JSON ingest, Excel fallback |
| [0006](adr/0006-rrf-k60-hybrid-hits.md) | Hybrid FTS + sqlite-vec, RRF k=60 |
| [0007](adr/0007-catalog-identity.md) | Identity is `(catalog_id, resource_id)` |
| [0008](adr/0008-dual-lane-guard.md) | MPP Payment then Unkey Bearer |
| [0009](adr/0009-ingester-per-catalog.md) | Ingester per Catalog |
| [0010](adr/0010-f2llm-query-prefix.md) | Asymmetric Instruct/Query prefix |
