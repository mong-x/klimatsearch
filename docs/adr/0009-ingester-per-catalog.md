# One Ingester per Catalog

Fetch-and-map is an Ingester: `Catalog() Catalog ID` plus `Fetch(ctx) (Batch, error)`. Boverket implements JSON (`GetAllResources/latest/{sv,en}/json` on `https://api.boverket.se/klimatdatabas`) with Excel fallback. BR25 and other file-only Catalogs implement a file parser and enter through `POST /admin/ingest/file?catalog=br25` (multipart), not a weekly HTTP poll. The apply/upsert/embed Runner stays Catalog-agnostic.
