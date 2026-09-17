# klimatsearch

Distributed search engine and MCP server for [Boverket Klimatdatabas](https://www.boverket.se/sv/klimatdeklaration/klimatdatabas/): ~200 generic construction resources in Swedish and English, with typical A1–A3 climate impact.

Data is from **Boverket Klimatdatabas**. You must cite Boverket as the source. This project is not affiliated with Boverket.

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
| GET | `/api/search?q=&vector=true\|false&rerank=true\|false&lang=sv\|en` |
| GET | `/api/resources?lang=` |
| GET | `/api/resources/{id}?lang=` |

JSON includes `"source": "Boverket Klimatdatabas"`.

## MCP

- Streamable HTTP: `POST/GET /mcp`
- Legacy SSE: `GET /mcp/sse`, messages `POST /mcp/messages?sessionid=` (also accepted on `/mcp/sse`)

Tools: `search_climate_data`, `get_resource_details`, `compare_materials`.

## Ingest

On start (default) and every `--ingest-interval` (default `168h`): probe `BOVERKET_API_BASE` JSON, then fall back to the public Excel files. Optional `BOVERKET_SUBSCRIPTION_KEY`. Changed rows are detected with a canonical ContentHash and re-embedded.

## Docker / kind

```bash
make docker          # klimatsearch:local
make e2e             # kind cluster + kustomize + HTTP probes
```

CI builds a linux binary artifact `klimatsearch-linux-amd64` and runs the kind e2e workflow.

## License

Apache-2.0. Klimatdatabas content remains Boverket's; cite Boverket when you use it.
