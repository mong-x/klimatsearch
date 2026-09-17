# klimatsearch

Distributed search engine and MCP server for [Boverket Klimatdatabas](https://www.boverket.se/sv/klimatdeklaration/klimatdatabas/): ~200 generic construction resources in Swedish and English, with typical A1–A3 climate impact.

Data is from **Boverket Klimatdatabas**. You must cite Boverket as the source. This project is not affiliated with Boverket.

**Software is ready for `make run` (fake embedder).** Accounts, ONNX weights, and production hosting are yours — start at **[docs/LAUNCH.md](docs/LAUNCH.md)**.

## Requirements

- Go 1.27
- CGO (`CGO_ENABLED=1`)
- gcc (and on Linux, `libsqlite3-dev`)
- Build tag `fts5` (mattn/go-sqlite3; `make test` / `make build` pass it)

## Quick start (fake embedder)

```bash
make run
# GET http://127.0.0.1:8080/healthz
# GET http://127.0.0.1:8080/api/search?q=betong&lang=sv
```

`--demo-fixture` loads `testdata/fixtures/resources.json` so a laptop demo works without the live API.

## Production model

Default production embedder is [codefuse-ai/F2LLM-v2-80M](https://huggingface.co/codefuse-ai/F2LLM-v2-80M) (hidden size 320). Weights are not in git.

```bash
make models          # huggingface-cli download; refuses to run in CI
# export to ONNX (see scripts/download-models.sh)
KLIMAT_EMBEDDER=onnx go run ./cmd/klimatsearch
```

The binary uses `onnx` when `{models}/{embedding-model}/model.onnx` exists, otherwise `fake`. `--embedder` / `KLIMAT_EMBEDDER` always wins.

Layout:

```
models/f2llm-v2-80m/model.onnx
models/f2llm-v2-80m/tokenizer.json
```

To swap models, point `--models` / `--embedding-model` at a new directory. Stored `meta.embedding_dim` must match; a mismatch disables vector search until you re-embed.

## REST

| Method | Path |
| --- | --- |
| GET | `/healthz` |
| GET | `/api/search?q=&vector=true\|false&rerank=true\|false&lang=sv\|en&databases=` |
| GET | `/api/resources?lang=&databases=` |
| GET | `/api/resources/{id}?lang=` |
| GET | `/api/resources/compare?a={id}&b={id}&unit=` |
| POST | `/admin/ingest/file?catalog=` |

`vector` and `rerank` default to false. Compare `unit` is optional. `databases` is a comma-separated list of Catalog IDs (`boverket`, `br25`); empty means all Catalogs. Resource `{id}` may be prefixed (`boverket:6000000000`) or a bare Resource ID when only one Catalog matches (409 if ambiguous). JSON includes Attribution `"source": "Boverket Klimatdatabas"` and `"catalog"` (Catalog ID). Hits include `match_source`: `fts`, `vector`, `both`, or `rerank`.

Admin file ingest is multipart field `file`, Guard-protected. Optional `KLIMAT_ADMIN_TOKEN` as `X-Admin-Token`. Unknown Catalog → 400.

Paid routes (`/api/*`, `/mcp`, `/admin/*`) go through a dual-lane Guard (ZeroClick signature, then Unkey Bearer). `GET /healthz` is public. When `UNKEY_ROOT_KEY` and `ZEROCLICK_SIGNING_SECRETS` are unset, the Guard allows all traffic (local/kind). If either is set, it is fail-closed (401, or 402 with `ZEROCLICK_STOREFRONT_URL`).

Hybrid search fuses FTS5 BM25 and sqlite-vec KNN with Reciprocal Rank Fusion (`k=60`, equal weights). Vector off → FTS only.

## MCP

- Streamable HTTP: `POST/GET /mcp`
- Legacy SSE: `GET /mcp/sse`, messages `POST /mcp/messages?sessionid=` (also accepted on `/mcp/sse`)

Tools: `search_climate_data` (optional `databases` array), `get_resource_details`, `compare_resources` (`id_a`, `id_b`, optional `unit`). Get/compare accept prefixed or bare Resource IDs.

## Ingest

On start (default) and every `--ingest-interval` (default `168h`): GET `{BOVERKET_API_BASE}/api/Klimat/v2/GetAllResources/latest/{sv,en}/json` (default base `https://api.boverket.se/klimatdatabas`), merge cultures, then fall back to the public Excel files on HTTP error, parse error, or an empty resources list (A1A3 of 0 is still usable JSON). Optional `BOVERKET_SUBSCRIPTION_KEY`. File-only Catalogs use `POST /admin/ingest/file?catalog=`. Changed rows are detected with a canonical ContentHash of names, descriptions, applicability, synonyms, A1A3, Declared unit, Conversions, Category, and DatasetVersion (not Resource ID or Catalog ID) and re-embedded.

## Docker / kind

```bash
make docker          # klimatsearch:local
make e2e             # kind cluster + kustomize + HTTP probes
```

CI builds a linux binary artifact `klimatsearch-linux-amd64` and runs the kind e2e workflow.

## License

Apache-2.0. Klimatdatabas content remains Boverket's; cite Boverket when you use it.
