# What you need to do

The Go product is complete for local/demo use: ingest, hybrid search (FTS + RRF), REST, MCP, Catalog identity, dual-lane Guard (fail-open until you add secrets), kind e2e, and CI unit tests. Fake embedder is the default so `make run` works without models.

Everything below needs **your accounts, files, or a machine with ONNX Runtime**. The binary will not guess these.

Work top to bottom. Each item is independently shippable.

---

## 0. Confirm the tree

```bash
cd ~/REPOS/klimatsearch
make test
make run          # fake embedder + fixture
# another terminal:
curl -s 'http://127.0.0.1:8080/healthz'
curl -s 'http://127.0.0.1:8080/api/search?q=betong&lang=sv'
```

Live Boverket ingest (no fixture):

```bash
go run -tags fts5 ./cmd/klimatsearch --embedder=fake --listen=:8080
# hits https://api.boverket.se/klimatdatabas/api/Klimat/v2/GetAllResources/latest/{sv,en}/json
```

Cite **Boverket Klimatdatabas** in any UI or paper that shows these numbers.

---

## 1. Production embeddings (F2LLM-v2-80M ONNX)

**Code is wired:** `FileTokenizer` uses `github.com/daulet/tokenizers` when you build `-tags tokenizers` (Makefile does this automatically if `third_party/tokenizers/libtokenizers.a` exists). `ONNX.Embed` runs the session, pools the last non-pad token (EOS), L2-normalizes. `EmbedQuery` adds the Instruct prefix. Tests skip if `models/f2llm-v2-80m/model.onnx` is missing (gitignored).

**Why you still do this on each machine:** ~160 MB safetensors + ~300 MB ONNX are not in git. `libonnxruntime` is a system package. `libtokenizers.a` is fetched per OS.

**Do this:**

1. ONNX Runtime:
   - macOS: `brew install onnxruntime`
   - Or set `ONNXRUNTIME_LIB` to the `.dylib` / `.so` ([releases](https://github.com/microsoft/onnxruntime/releases)).
2. Tokenizers C lib:

   ```bash
   ./scripts/fetch-libtokenizers.sh
   ```

3. Weights + tokenizer + ONNX (Hugging Face; no `huggingface-cli` required if you have `uv`):

   ```bash
   make models
   python scripts/export-f2llm-onnx.py   # or: uv run --with 'optimum[onnxruntime]' --with transformers --with torch python scripts/export-f2llm-onnx.py
   ```

   Expected:

   ```
   models/f2llm-v2-80m/model.onnx
   models/f2llm-v2-80m/tokenizer.json
   third_party/tokenizers/libtokenizers.a
   ```

4. Re-ingest so vectors match labeled `EmbeddingText()`:

   ```bash
   rm -f data/klimat.db
   make build
   KLIMAT_EMBEDDER=onnx ./bin/klimatsearch
   ```

5. Score `testdata/golden-queries.json` (hit@1 / sibling inversions). Do **not** use the Fake embedder for that table.

Protocol: [ADR-0010](adr/0010-f2llm-query-prefix.md), [research note](research/2026-09-17-boverket-embedding-optimization.md).

---

## 2. Paid traffic (Unkey + ZeroClick)

**Why you:** dashboard accounts, Stripe, signing secrets. Local/kind **AllowAll** when both secrets are unset (ADR-0008).

**Developer lane (REST / SaaS)**

1. Create an [Unkey](https://unkey.com) workspace and API.
2. Connect Unkey metered billing to **Stripe**.
3. Create keys for customers.
4. Set `UNKEY_ROOT_KEY` on the server.
5. Clients send `Authorization: Bearer <key>`.
6. Use the current SDK: `github.com/unkeyed/sdks/api/go/v2` (already in `go.mod`). Do not use archived `unkey-go`.

**Agent lane (MCP / x402 / MPP)**

1. Create a [ZeroClick.ai](https://zeroclick.ai) seller, product, and storefront URL.
2. Copy the **signing secret** (`zcsec_…`) and API key with `usage:read` / `usage:write`.
3. Set:
   - `ZEROCLICK_SIGNING_SECRETS`
   - `ZEROCLICK_API_KEY`
   - `ZEROCLICK_STOREFRONT_URL` (used on HTTP 402)
4. Point agents at the ZeroClick pay URL, not your raw origin.

**The moment either Unkey or ZeroClick secret is set, the Guard is fail-closed.** `/healthz` stays public. Paid: `/api/*`, `/mcp`, `/admin/*`.

Optional: `KLIMAT_ADMIN_TOKEN` as header `X-Admin-Token` on `POST /admin/ingest/file`.

---

## 3. Denmark BR25 (and other file Catalogs)

**Why you:** no public API; you need the official Excel/CSV and a column map.

1. Obtain a BR25 workbook.
2. Implement mapping in `internal/ingest/file.go` (`CatalogBR25` currently returns `ErrCatalogNotImplemented`).
3. Upload:

   ```bash
   curl -F file=@br25.xlsx \
     -H "X-Admin-Token: $KLIMAT_ADMIN_TOKEN" \
     -H "Authorization: Bearer $UNKEY_KEY" \
     'http://127.0.0.1:8080/admin/ingest/file?catalog=br25'
   ```

4. Search: `GET /api/search?q=beton&databases=br25` (and `boverket,br25` for both Catalogs).

Identity is already `(catalog_id, resource_id)`; vec0 id is `br25:…`.

---

## 4. Production Kubernetes / AWS

Kind e2e uses `klimatsearch:e2e`, `imagePullPolicy: Never`, fake embedder, fixture ingest.

**You:**

1. Build and push a real image (include `models/` in the image or a volume; do not git LFS the weights unless you choose to).
2. Set `image:` + `imagePullPolicy: IfNotPresent` in `deploy/k8s/deployment.yaml`.
3. Replace the Ingress stub (`deploy/k8s/ingress.yaml`) with your ALB / ingress class, host, TLS.
4. Attach secrets: `UNKEY_ROOT_KEY`, ZeroClick vars, `ONNXRUNTIME_LIB`, `KLIMAT_EMBEDDER=onnx`.
5. PVC is optional: ~230 rows rebuild on boot via the Boverket Ingester. Use a PVC if you do not want to re-embed on every pod start.
6. Memory: ONNX 80M needs more than the e2e `512Mi` limit — raise it (1–2 Gi is a sane starting point).
7. CGO: image already builds with `CGO_ENABLED=1` and `-tags fts5`.

---

## 5. Optional / later

| Item | Notes |
| --- | --- |
| Reranker ONNX (`BAAI/bge-reranker-v2-m3`) | `--reranker=onnx` is a compile stub; leave `none` until embeddings work. |
| Subscription key | Live Boverket v2 JSON worked without `BOVERKET_SUBSCRIPTION_KEY`; keep the header if APIM starts requiring it. |
| MCP client config | Point Claude/Cursor at `/mcp` (streamable) or `/mcp/sse`. |
| Golden set expansion | Add Fabriksbetong C-class Queries after ONNX works. |
| Graphify | `graphify extract . --code-only` if you want a code graph; `graphify-out/` is gitignored. |

---

## Checklist

```
[ ] make test && make run
[ ] Live ingest from api.boverket.se (no --demo-fixture)
[ ] brew install onnxruntime (or ONNXRUNTIME_LIB)
[ ] ./scripts/fetch-libtokenizers.sh
[ ] make models && python scripts/export-f2llm-onnx.py
[ ] rm data/klimat.db && KLIMAT_EMBEDDER=onnx make build && ./bin/klimatsearch
[ ] Score testdata/golden-queries.json with ONNX (not Fake)
[ ] Score testdata/golden-queries.json
[ ] Unkey root key + Stripe
[ ] ZeroClick seller + storefront URL
[ ] Confirm 401/402 with secrets set, 200 on /healthz without
[ ] BR25 file + FileIngester mapping (when you have the workbook)
[ ] Production image, Ingress, memory, secrets
[ ] Attribution “Boverket Klimatdatabas” on every public surface
```
