# Klimatsearch

Glossary for climate-data retrieval: Resources in a Catalog, how they change, and how a Query becomes Hits.

## Language

**Catalog**:
A national climate-data publication klimatsearch can ingest (Boverket Klimatdatabas, BR25).
_Avoid_: source, namespace, database

**Catalog ID**:
The stable code for a Catalog (`boverket`, `br25`).
_Avoid_: source_id, namespace, database name

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
The typical GWP-GHG climate impact of a Resource for life-cycle modules A1–A3, in kg CO2e per Declared unit.
_Avoid_: GWP, carbon footprint, emission factor, conservative A1-A3

**DatasetVersion**:
The Klimatdatabas publication a Resource was ingested from (for example 02.07.000).
_Avoid_: version (bare), schema version

**ContentHash**:
The identity of a Resource's names, descriptions, Applicability, Synonyms, A1A3, Declared unit, Conversion, Category, and DatasetVersion, used to detect whether the Resource changed. Resource ID and Catalog ID are identity, not content.
_Avoid_: checksum, etag, fingerprint, SHA-256

**Query**:
A natural-language request for Resources, in Swedish or English.
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
The dual-lane check on paid endpoints: MPP `Authorization: Payment` (`github.com/tempoxyz/mpp-go`), then Unkey Bearer API key. Unpaid MPP traffic is HTTP 402.
_Avoid_: auth middleware, billing proxy

**Ingester**:
The fetch-and-map of one Catalog into Resources (JSON API, Excel, or an uploaded file).
_Avoid_: crawler, importer, connector

**Comparison**:
A1A3 of two Resources expressed in one shared unit, using Conversion when the caller asks or when Declared units already agree.
_Avoid_: material compare, delta (bare)

**Embedder**:
The mapping from Resource text or a Query to a vector.
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
