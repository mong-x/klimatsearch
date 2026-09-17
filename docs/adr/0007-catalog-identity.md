# Catalog identity, not a source column

Attribution already means the Boverket citation, and Hit `match_source` means fts/vector/both/rerank. A second climate publication (BR25) is a **Catalog** with a Catalog ID (`boverket`, `br25`). Resource identity is `(catalog_id, resource_id)`; sqlite-vec's single PK is the prefixed document id `boverket:6000000000`. Queries may filter Catalogs (`databases=` on REST is the HTTP name for Catalog IDs). Do not name the column `source`.
