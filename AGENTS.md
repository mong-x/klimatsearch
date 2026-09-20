# Agents

Go 1.27. CGO is required (`CGO_ENABLED=1`) for mattn/go-sqlite3, sqlite-vec, and onnxruntime_go.

HTTP routing uses `net/http.ServeMux` only (Go 1.22 method + path values). Do not add Chi, Gin, or Echo.

Operator UI is `internal/web` (`html/template` + `go:embed`, no React). `GET /`, `/data`, `/data/embed/{id}`, `/resources/{id}`, `/compare`, `/ingest`, `/connect` on the same mux as REST and MCP. Data inspects allowlisted SQLite objects only. Danish Catalog is `dkbr` (`br25` alias); DatasetVersion is per Catalog. File ingest uses a Column map; JSON upsert is `POST /admin/resources`. Query construction is `search.FromURL` / `FromMCP`. Hit.Source is retrieval; rerank is `Reranked` + `RerankScore`.

Glossary is `CONTEXT.md`. Human launch work (ONNX, hosted Unkey/MPP, BR25, AWS) is `docs/LAUNCH.md`. Consumer agents connecting to or self-hosting klimatsearch: `llms.txt` then `docs/AGENT-SELFHOST.md` (cost, MCP/REST, HPA). Default build is self-hosted (`-tags fts5` only): no Unkey, no MPP. Hosted Guard is `-tags hosted`. Do not add architecture decision records, research notes, or a PRD to git. `go mod tidy -tags "fts5 hosted"` so hosted module deps stay in go.mod.

Do not commit `/data/*.db`, `/models/**` weights, `*.onnx`, or `*.safetensors`. Tests and default CI use the fake Embedder; they must not require ONNX weights or libonnxruntime.

Call `sqlite_vec.Auto()` before opening SQLite. Store embedding dimension in `meta`; a dim mismatch disables vector search rather than mixing vectors.

mattn/go-sqlite3 compiles FTS5 only with `-tags fts5`. Use `make test` / `make build`, or `CGO_ENABLED=1 go test -tags fts5 ./...`.

Ingest prefers the Boverket JSON API, then the public Excel files. Tests inject a Fetcher or `--demo-fixture` and never hit the network.
