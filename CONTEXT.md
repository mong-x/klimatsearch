# Klimatsearch

Glossary for the Boverket Klimatdatabas search engine: the terms used in APIs, ingest, and ranking.

## Language

**Resource**:
A generic construction product or energy carrier from Boverket Klimatdatabas, identified by a stable Resource ID.
_Avoid_: material, product, item

**ContentHash**:
A SHA-256 digest of a Resource's canonical fields, used to detect whether the Resource changed since last ingest.
_Avoid_: checksum, etag, fingerprint

**A1A3**:
The typical GWP-GHG climate impact of a Resource for life-cycle modules A1–A3, in kg CO2e per declared unit.
_Avoid_: GWP, carbon footprint, emission factor, conservative A1-A3

**Conversion**:
A factor that restates a Resource's declared unit in another unit (for example kg per m³).
_Avoid_: density, multiplier, unit map

**Embedder**:
A component that turns Resource text into a fixed-dimension vector for nearest-neighbour search.
_Avoid_: encoder, model, embed

**Reranker**:
A component that reorders SearchEngine results after retrieval, using the query and candidate Resources.
_Avoid_: ranker, scorer

**SearchEngine**:
The hybrid retrieval component that answers a query from FTS, vectors, and an optional Reranker.
_Avoid_: index, retriever, finder

**Klimatdatabas**:
Boverket's published climate database of generic construction Resources, cited as "Boverket Klimatdatabas".
_Avoid_: climate DB, Boverket API, LCA database
