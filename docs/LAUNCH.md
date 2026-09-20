# What you need to do

The Go product is complete for local/demo use: ingest, hybrid search (FTS + RRF), REST, MCP, Catalog identity, dual-lane Guard (fail-open until you add secrets), kind e2e, and CI unit tests. Fake embedder is the default so `make run` works without models. Public profile: [README](../README.md) and [GitHub Pages](https://mong-x.github.io/klimatsearch/).

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

Self-hosters (AWS, Docker, a VM): follow **[SELFHOST.md](SELFHOST.md)** — F2LLM embedder, optional BGE reranker, `check-models.sh`, Linux ONNX Runtime.

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

5. Score the labeled set: `make eval-golden` (Hit@1 / Hit@3 / sibling inversions / INT8 vs fp32 agreement). Do **not** use the Fake embedder. Re-ingest with `KLIMAT_EMBEDDER=onnx` first.

Documents use labeled bilingual `EmbeddingText` (no Instruct prefix). Queries use `Instruct:…\nQuery:`. Do not score quality with the Fake embedder.

---

## 2. Paid traffic (Unkey + MPP)

This section is only for **our** process, built with `make build-hosted` (`-tags hosted`). `make build` / `make run` is the self-hosted binary: no Unkey SDK, no mpp-go, Guard is a pass-through. Docker default is self-hosted; production image: `docker build --build-arg GO_TAGS="fts5 hosted"`.

**Why you:** dashboard accounts, Stripe, Tempo recipient. Local/kind **AllowAll** when Unkey and MPP secrets are unset.

**Developer lane (REST / SaaS)**

You created a **keyspace**. klimatsearch needs two different kinds of Unkey material — do not mix them, and do not paste them into chat.

| What | Where in Unkey | Where in klimatsearch | Who uses it |
| --- | --- | --- | --- |
| **Root key** | Dashboard → Settings → Root Keys (starts with `unkey_`) | Server env `UNKEY_ROOT_KEY` | Only the klimatsearch process, to call VerifyKey |
| **API keys** | Your keyspace → Create key (often `sk_…`) | **Not** in server env. Callers send `Authorization: Bearer <api key>` | Apps, curl, klimatdeklaration software |

A keyspace alone is not enough: create at least one **root key** for the server and one **API key** in that keyspace to test.

Put secrets in a gitignored `.env` next to `go.mod` (copy `.env.example`) or export them in the shell. The process loads `.env` on startup; already-exported variables win.

```bash
cp .env.example .env
# edit .env — UNKEY_ROOT_KEY=unkey_...
KLIMAT_LISTEN=:8081 KLIMAT_EMBEDDER=onnx ./bin/klimatsearch
```

Test (use **your API key**, not the root key):

```bash
curl -sS -H "Authorization: Bearer sk_YOUR_API_KEY" \
  'http://127.0.0.1:8081/api/search?q=betong&lang=sv'
```

`/healthz` stays public. After `UNKEY_ROOT_KEY` is set, other `/api/*` routes return 401 without a valid Bearer API key.

Stripe metered billing is configured in the Unkey dashboard, not in this repo. SDK: `github.com/unkeyed/sdks/api/go/v2`.

**Agent lane (MCP / MPP)** — official [mpp-go](https://github.com/tempoxyz/mpp-go). There is no MPP dashboard account.

1. Generate a server HMAC (not from Unkey):

   ```bash
   openssl rand -hex 32
   ```

2. Create a Tempo address you control (this is `MPP_RECIPIENT`). Testnet:

   ```bash
   npx mppx account create --network testnet
   npx mppx account fund --network testnet
   ```

   Or [Tempo Wallet](https://wallet.tempo.xyz). Use that address as the payee.

3. Put them in `.env` (do not paste them into chat):

   - `MPP_SECRET_KEY` — the openssl hex; server only; signs challenge IDs
   - `MPP_RECIPIENT` — your `0x…` Tempo address (required with the secret)
   - optional `MPP_RPC_URL` (default `https://rpc.moderato.tempo.xyz` = Tempo testnet)
   - optional `MPP_REALM` (default `klimatsearch`)
   - optional `MPP_AMOUNT` (default `0.01`, dollars)

   Mainnet instead of testnet: `MPP_RPC_URL=https://rpc.tempo.xyz`.

4. Restart klimatsearch. Unpaid `/api/*` and `/mcp` return **HTTP 402** with `WWW-Authenticate: Payment`.

   ```bash
   curl -sS -D- 'http://127.0.0.1:8081/api/search?q=betong&lang=sv' | head
   npx mppx validate http://127.0.0.1:8081
   npx mppx 'http://127.0.0.1:8081/api/search?q=betong&lang=sv'
   ```

   Agents retry with `Authorization: Payment …` (`github.com/tempoxyz/mpp-go/pkg/client`, or `npx mppx`). Developer `Authorization: Bearer` Unkey keys still work on the same origin.

**The moment Unkey or MPP secret is set, the Guard is fail-closed.** `/healthz` stays public. Paid: `/api/*`, `/mcp`, `/admin/*`.

**Webhook** — optional. Set `KLIMAT_WEBHOOK_URL` (and `KLIMAT_WEBHOOK_SECRET` to HMAC the body). klimatsearch POSTs:

- `catalog.changed` when ingest upserts a Resource whose ContentHash changed
- `catalog.unreachable` when Boverket JSON and Excel both fail (API down)
- `ingest.failed` when boot or weekly ingest errors for any other reason

The same events are stored in SQLite (`ingest_events`, last 200) and listed on `GET /data`. Every enabled destination is notified: Slack (`hooks.slack.com` → `{"text":…}`), Discord (`discord.com/api/webhooks` → `{"content":…}`), anything else JSON Event. Add several in the console (`GET /webhooks`) or comma-separate `KLIMAT_WEBHOOK_URL`. HMAC is skipped for Slack and Discord. One destination failing does not skip the others. Boverket has no inbound hook.

Optional: `KLIMAT_ADMIN_TOKEN` as header `X-Admin-Token` on `POST /admin/ingest/file`.

---

## 3. Denmark BR25 (and other file Catalogs)

**Why you:** no public API; you need the official Excel/CSV and a column map.

1. Obtain a Danish BR workbook (BR18, BR25, or later). Catalog ID is `dkbr`; DatasetVersion is `BR18` / `BR25` / …
2. Preview columns (`POST /admin/ingest/preview` or console Preview), map headers onto the Resource schema, then apply — or POST already-mapped JSON to `/admin/resources`.
3. Console: `GET /ingest` (catalog **dkbr**, DatasetVersion field). REST:

   ```bash
   curl -F file=@br.xlsx -F version=BR25 -F map.id=ID -F map.a1a3=GWP \
     -H "X-Admin-Token: $KLIMAT_ADMIN_TOKEN" \
     'http://127.0.0.1:8080/admin/ingest/file?catalog=dkbr'
   ```

4. Search: `GET /api/search?q=beton&databases=dkbr` (`br25` still aliases to `dkbr`).

Identity is already `(catalog_id, resource_id)`; vec0 id is `br25:…`.

---

## 4. Production Kubernetes / AWS

Kind e2e uses `klimatsearch:e2e`, `imagePullPolicy: Never`, fake embedder, fixture ingest.

**You:**

1. Follow [SELFHOST.md](SELFHOST.md). Build and push a real image (mount `models/` or a private layer; do not git the weights). `KLIMAT_EMBEDDER=onnx`. Reranker: `KLIMAT_RERANKER=onnx` (BGE-m3) or `none`. Example manifest: `deploy/k8s/deployment.onnx.yaml`.
2. Set `image:` + `imagePullPolicy: IfNotPresent` in `deploy/k8s/deployment.yaml`.
3. Replace the Ingress stub (`deploy/k8s/ingress.yaml`) with your ALB / ingress class, host, TLS.
4. Attach secrets: `UNKEY_ROOT_KEY`, `MPP_SECRET_KEY`, `MPP_RECIPIENT`, `ONNXRUNTIME_LIB`, `KLIMAT_EMBEDDER=onnx`.
5. PVC is optional: ~230 rows rebuild on boot via the Boverket Ingester. Use a PVC if you do not want to re-embed on every pod start.
6. Memory: embedder-only **1–2 Gi**; embedder + BGE **4 Gi** (`deployment.onnx.yaml`); zerank-1-small **8 Gi**. Kind e2e’s `512Mi` is fake-embedder only.
7. CGO: image already builds with `CGO_ENABLED=1` and `-tags fts5`.

---

## 5. Optional / later

| Item | Notes |
| --- | --- |
| Reranker ONNX (BGE-m3, self-host default) | Same OSS binary. `KLIMAT_RERANKER=onnx`, REST `rerank=true`, MCP reranks when loaded. Compose: `deploy/compose.onnx.yml`. ~4 GiB. |
| Reranker ONNX (zerank-1-small) | `python scripts/export-zerank-onnx.py` then `KLIMAT_RERANKER_MODEL=zerank-1-small`. Apache 1.7B; ~8 GiB. Do not load zerank-2 in-process. |
| INT8 ONNX | `python scripts/quantize-onnx.py models/<dir>` then default `KLIMAT_ONNX_QUANT=auto` loads `model.int8.onnx`. |
| Subscription key | Live Boverket v2 JSON worked without `BOVERKET_SUBSCRIPTION_KEY`; keep the header if APIM starts requiring it. |
| MCP client config | Point Claude/Cursor at `/mcp` (streamable) or `/mcp/sse`. |
| Golden eval | `make eval-golden` scores `testdata/golden-queries.json` (44 sibling/exact/cross-lingual queries). INT8 and zerank lanes appear when those ONNX files exist. |
| Graphify | `graphify extract . --code-only` if you want a code graph; `graphify-out/` is gitignored. |

---

## Checklist

```
[ ] make test && make run
[ ] Live ingest from api.boverket.se (no --demo-fixture)
[ ] brew install onnxruntime (or ONNXRUNTIME_LIB)
[ ] ./scripts/fetch-libtokenizers.sh
[ ] make models && python scripts/export-f2llm-onnx.py && python scripts/quantize-onnx.py models/f2llm-v2-80m
[ ] rm data/klimat.db && KLIMAT_EMBEDDER=onnx make build && ./bin/klimatsearch
[ ] `make eval-golden` (F2LLM-ingested DB; Fake numbers are not a quality signal)
[ ] Unkey **root key** in `.env` as `UNKEY_ROOT_KEY` (not the keyspace API key)
[ ] Create a customer **API key** in the keyspace; curl with `Authorization: Bearer`
[ ] MPP_SECRET_KEY (`openssl rand -hex 32`) + MPP_RECIPIENT (Tempo address) in `.env`
[ ] Unpaid search returns 402 + WWW-Authenticate: Payment; `npx mppx` can pay on testnet
[ ] Confirm 401/402 with secrets set, 200 on /healthz without
[ ] BR25 file via operator `GET /ingest` or `POST /admin/ingest/file?catalog=br25`
[ ] Production image, Ingress, memory, secrets
[ ] Attribution “Boverket Klimatdatabas” on every public surface
```
