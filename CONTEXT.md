# Klimatsearch

Glossary for climate-data retrieval: Resources in a Catalog, how they change, and how a Query becomes Hits.

## Language

**Catalog**:
A national climate-data publication klimatsearch can ingest (Boverket Klimatdatabas, Danish BR).
_Avoid_: source, namespace, database

**Catalog ID**:
The stable code for a Catalog (`boverket`, `dkbr`). `br25` is a deprecated alias for `dkbr`.
_Avoid_: source_id, namespace, database name, BR25 (as a Catalog)

**Resource**:
A generic construction product or energy carrier in a Catalog, identified by Catalog ID plus Resource ID.
_Avoid_: material, product, item

**Resource ID**:
The identifier the Catalog assigns to a Resource (Boverket's `ResourceId`, for example 6000000000). Unique only inside that Catalog.
_Avoid_: id (bare), key, material ID

**Category**:
The Catalog's grouping of Resources. Boverket exposes a numeric Category Code (`code`) and a title (for example Betong).
_Avoid_: type, class, group

**Culture**:
Boverket API language path (`sv`, `en`); payloads use `sv-se` and `en-gb`. Maps to Language.
_Avoid_: locale, cultureInfo

**Synonym**:
An alternate name listed on a Resource (Boverket `Synonyms`).
_Avoid_: alias, tag

**Applicability**:
Where a Resource is typically used (Boverket `TechnologicalApplicability` / use of product).
_Avoid_: usage, application

**Declared unit**:
The unit A1A3 is expressed per (for example kg).
_Avoid_: unit (bare), base unit

**Conversion**:
A factor that restates a Resource's Declared unit in another unit (for example kg per m³).
_Avoid_: density, multiplier, unit map

**A1A3**:
The typical GWP-GHG climate impact of a Resource for life-cycle modules A1–A3, in kg CO2e per Declared unit. Compare uses this value.
_Avoid_: GWP, carbon footprint, emission factor, conservative A1-A3

**A1A3 Conservative**:
Boverket's conservative A1–A3 (~25% above typical), used in climate declarations. Not the Compare value.
_Avoid_: declaration GWP (bare)

**A4**:
GWP-GHG for transport to the construction site. Present on construction products, not energy carriers.
_Avoid_: transport emissions (bare)

**A5.1**:
GWP-GHG for construction-site waste (module A5.1). Present on construction products, not energy carriers.

**Details**:
The rest of Boverket's Resource payload klimatsearch surfaces (conservative A1–A3, A4, A5.1, factors, biogenic carbon, service life, comments, BK04, transport legs).
_Avoid_: raw JSON dump

**DatasetVersion**:
The publication of a Catalog a Resource was ingested from (Boverket `02.07.000`; Danish BR `BR18`, `BR25`). Unique per Catalog, not a Catalog ID.
_Avoid_: version (bare), schema version, BR25 (as identity)

**Column map**:
Caller-supplied pairing of a file header to a Resource field before ingest. Danish workbooks vary; the Ingester does not guess new names.
_Avoid_: auto-detect, LLM mapping (inside the process)

**Resource batch**:
Already-mapped Resources applied by the ingest Runner (JSON upsert). File bytes are not required.
_Avoid_: bulk insert, raw SQL load

**ContentHash**:
The identity of a Resource's names, descriptions, Applicability, Synonyms, A1A3, Declared unit, Conversion, Category, and DatasetVersion, used to detect whether the Resource changed. Resource ID and Catalog ID are identity, not content.
_Avoid_: checksum, etag, fingerprint, SHA-256

**Query**:
A request for Hits: text, Language, optional Catalogs, and whether vector retrieval and the Reranker run.
_Avoid_: search string, prompt, q

**Hit**:
A Resource returned for a Query, with a score and how it was matched (`fts`, `vector`, `both`, or `rerank`).
_Avoid_: SearchResult, document, match

**Language**:
Swedish (`sv`) or English (`en`). FTS uses the Resource fields of that Language; vector retrieval is cross-lingual.
_Avoid_: locale, lang flag

**Attribution**:
The required citation of a Catalog as origin, for example "Boverket Klimatdatabas".
_Avoid_: source (bare — overloaded with how a Hit was matched and with Catalog)

**Guard**:
Endpoint check wrapping the whole mux. The **hosted** build (`-tags hosted`) runs the dual-lane Guard: MPP `Authorization: Payment`, then Unkey Bearer. Both builds honor self-managed static keys (`KLIMAT_API_KEYS`): `Authorization: Bearer`, `X-Api-Key`, or `?api_key=`; unset means no gating. `/healthz`, `/openapi.yaml`, `/docs` stay public.
_Avoid_: auth middleware, billing proxy

**Ingester**:
The fetch-and-map of one Catalog into Resources (JSON API, Excel, or an uploaded file).

**Webhook**:
The outbound POST klimatsearch sends to `KLIMAT_WEBHOOK_URL`. `catalog.changed` after a ContentHash upsert; `catalog.unreachable` when Boverket JSON and Excel both fail; `ingest.failed` when a scheduled or boot ingest errors. Events are also stored in SQLite (last 200).
_Avoid_: listener, callback, inbound Boverket hook
_Avoid_: crawler, importer, connector

**Comparison**:
Climate impact of two Resources in one shared unit (typical A1A3 by default; conservative, A4, or A5.1 when asked), using Conversion when the caller asks or when Declared units already agree.
_Avoid_: material compare, delta (bare)

**Embedder**:
The mapping from Resource text (`Embed`) or a Query (`EmbedQuery`) to a vector. ONNX applies the F2LLM Instruct prefix only on EmbedQuery.
_Avoid_: encoder, model, embed, component

**Reranker**:
The reordering of Hits after retrieval, using the Query and candidate Resources.
_Avoid_: ranker, scorer, component

**SearchEngine**:
Hybrid retrieval of Hits from Klimatdatabas for a Query.
_Avoid_: index, retriever, finder, component

**Klimatdatabas**:
Boverket's Catalog of generic construction Resources, cited as "Boverket Klimatdatabas".
_Avoid_: climate DB, Boverket API, LCA database
