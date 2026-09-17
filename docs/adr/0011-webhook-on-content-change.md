# Outbound webhook when ContentHash changes

Ingest already skips unchanged Resources. Subscribers (LCA tools, caches, agents) need to know when a Catalog actually moved.

After a successful `Runner.Apply` that upserted at least one row, klimatsearch POSTs `catalog.changed` to `KLIMAT_WEBHOOK_URL` (comma-separated list). Unchanged ingest (all hashes match) does **not** fire. Webhook HTTP errors are logged and do not fail ingest.

Body is JSON: event, catalog, version, ingest_origin, seen/upserted/skipped, ids (`catalog:id` of changed Resources), source (attribution), at (UTC). If `KLIMAT_WEBHOOK_SECRET` is set, `X-Klimat-Signature: sha256=<hex HMAC-SHA256 of the raw body>`. `X-Klimat-Event: catalog.changed`.

Boverket has no inbound webhook; this is klimatsearch notifying its own clients after the weekly (or admin) ingest.
