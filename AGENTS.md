# Agents

Go 1.27. CGO is required (`CGO_ENABLED=1`) for mattn/go-sqlite3, sqlite-vec, and onnxruntime_go.

HTTP routing uses `net/http.ServeMux` only (Go 1.22 method + path values). Do not add Chi, Gin, or Echo.

Glossary is `CONTEXT.md`. Human launch work (ONNX, Unkey, MPP, BR25, AWS) is `docs/LAUNCH.md`. Do not add architecture decision records, research notes, or a PRD to git.

Do not commit `/data/*.db`, `/models/**` weights, `*.onnx`, or `*.safetensors`. Tests and default CI use the fake Embedder; they must not require ONNX weights or libonnxruntime.

Call `sqlite_vec.Auto()` before opening SQLite. Store embedding dimension in `meta`; a dim mismatch disables vector search rather than mixing vectors.

mattn/go-sqlite3 compiles FTS5 only with `-tags fts5`. Use `make test` / `make build`, or `CGO_ENABLED=1 go test -tags fts5 ./...`.

Ingest prefers the Boverket JSON API, then the public Excel files. Tests inject a Fetcher or `--demo-fixture` and never hit the network.
