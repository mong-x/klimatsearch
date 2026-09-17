# Boverket embedding optimization for klimatsearch

Date: 2026-09-17  
Corpus: Boverket Klimatdatabas version **02.07.000** (Excel pair downloaded 2026-09-17, not committed)  
Planned embedder: `codefuse-ai/F2LLM-v2-80M`  
This note is research only. It does not change Go code.

Workbooks inspected (official, not in git):

- Swedish: <https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-klimatdatabas-version-02.07.000-sv-se.xlsx>
- English: <https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-climate-database-version-02.07.000-en-gb.xlsx>

## Executive recommendation

Keep **one 320-d cosine vector per Resource**. Embed a **labeled bilingual document**, not the current space-joined names+descriptions. Prefix **queries only**. Pool **last token / EOS**, then L2-normalize. Do not put A1–A3 magnitudes in the document string.

**Document string** (no instruction prefix):

```text
name_sv: {NameSV}
name_en: {NameEN}
category_sv: {CategorySV}
category_en: {CategoryEN}
unit: {declared unit, e.g. kg}
{DescriptionSV}
{DescriptionEN}
```

Skip empty fields. Do not concatenate with a single space across unlabeled languages.

**Query string** (instruction prefix, official F2LLM-v2 format):

```text
Instruct: Given a search query, retrieve the matching generic construction product or energy carrier from Boverket Klimatdatabas.
Query: {raw user query}
```

If you want zero invention, use the model-card default instead:

```text
Instruct: Given a question, retrieve passages that can help answer the question.
Query: {raw user query}
```

**Pooling / dim:** last-token (EOS) pooling, `hidden_size` 320, cosine, L2-normalize. Do not truncate Matryoshka dims at n≈230. Do not apply the Qwen3 chat template.

**Why this, in one paragraph.** The corpus is 230 unique-ID rows. Names are unique but tightly clustered (22 `Fabriksbetong, husbyggnad …` grades). FTS already covers exact tokens in the requested language; the vector’s job is paraphrase, translation, and “isolering”-style category queries. F2LLM-v2 is an instruction-tuned decoder embedder: queries get `Instruct: …\nQuery:`, documents get nothing. Category and declared unit are short, always present, and missing from both current `EmbeddingText()` and FTS. A1–A3 numbers belong in structured compare/filter, not in the embedding — subword models do not rank by magnitude reliably, and typical vs conservative is a policy choice, not a retrieval key.

Measure this on a 10–20 query golden set in SV and EN **with the real ONNX model**. The fake SHA-256 embedder is not a quality signal.

### Family-text addendum (same workbooks)

`Teknisk beskrivning` is **not** unique per Resource: 25 texts are reused across **157** of 230 rows. All **22** Fabriksbetong rows share one description; 6 Glasull rows share two. The tokens that separate siblings (`C25/30`, `klimatförbättrad`, `lösull`, `vindsbjälklag`) exist **only in the product name**. Last-token pooling therefore needs the name **adjacent to EOS** — repeat `{NameSV} / {NameEN}` as the last line of the document.

`Produktens användningsområde` / `Use of product` (227 nonempty) sometimes differs inside a family (Glasull ljudisolering). It is **not ingested today**. Do not embed A4 transport/fuel columns: 154 rows share the same diesel last-mile prose and would hub toward energy Resources.

Ingest gaps before this template can be built: bilingual Category (Resource has one Category field), use-of-product, and the trailing name line in `EmbeddingText()`.

---

## Boverket corpus facts

Sources for this section: the two official 02.07.000 workbooks above; Boverket’s English climate-database page; Boverket’s open-data page; klimatsearch ingest/search code.

### Official description of the dataset

Boverket publishes generic climate data for climate declarations. Construction-product A1–A3 values used in the declaration are **conservative** (~25 % above typical). Typical A1–A3 values exist for material comparison and **must not** be used in the final declaration. Energy and fuel use **typical** values, not conservative. The database is bilingual SV/EN. Open data is “över 200 generiska byggprodukter”. Version 02.07.000 was published 21 January 2026.

- <https://www.boverket.se/en/start/laws-and-regulations/climate-declaration/climate-database/>
- <https://www.boverket.se/sv/om-boverket/oppna-data/boverkets-klimatdatabas/>
- <https://www.boverket.se/sv/klimatdeklaration/om-klimatdeklaration/nyheter/ny-version-klimatdatabas/>

klimatsearch already stores **typical** A1–A3 for products (and energy typical), not the conservative column. That matches ADR-0005.

### Workbook structure

Both files are one sheet, 231 rows including header (230 data rows), 49 columns. Resource IDs are a 1–1 match across languages (230/230). Version is `02.07.000` on every row.

| | Swedish workbook | English workbook |
| --- | --- | --- |
| Sheet | `Boverkets klimatdatabas` | `Boverkets climate database` |
| ID | `Resurs-ID` | `Resource ID` |
| Name | `Produktnamn` | `Product name` |
| Category | `Kategori` | `Category` |
| Description | `Teknisk beskrivning` | `Technical description` |
| Declared unit | `Enhet för klimatpåverkan` | `Unit for climate impact` |
| Typical A1–A3 (products) | `A1-A3 byggproduktens klimatpåverkan GWP-GHG, typiskt värde` | `A1-A3 building product's climate impact GWP-GHG, typical value` |
| Conservative A1–A3 | `A1-A3 klimatpåverkan, konservativt värde` | `A1-A3 resources climate impact, conservative value` |
| Conservative factor | `A1-A3 faktor för konservativa värden` | `A1-A3 conservative factor` |
| Energy typical | `Energislagets klimatpåverkan GWP-GHG, typiskt värde` | `Energy product's climate impact GWP-GHG, typical value` |
| Conversion | `Omräkningsfaktor` + `Enhet för omräkningsfaktor` | `Conversion factor` + `Unit for conversion factor` |
| Coarse synonym-like | `Komponentnamn enligt BK04` | `Component name for BK04` |

There is **no synonym column**. BK04 names are a 63-value codebook (e.g. 14 glass-wool rows share `Mineralull`), not user-facing aliases.

Other columns exist (A4, A5, service life, biogenic carbon, transport legs, market text). They are not in klimatsearch’s `Resource` and are out of scope for the vector.

### Resource counts

| Slice | n | Source |
| --- | --- | --- |
| Rows in each workbook | 230 | Excel 02.07.000 |
| Unique Resource IDs, SV∩EN | 230 | merge on ID |
| Construction products (typical A1–A3 filled) | 208 | typical column nonempty; conservative also 208 |
| Energy and fuel (energy typical filled) | 22 | energy column nonempty; disjoint from the 208 |
| Conservative factor on products | 1.25 on all 208 | Excel; matches Boverket’s “approximately 25 percent” |

Boverket’s “över 200” is the construction-product set. klimatsearch’s ~200 in the PRD is the same order; the live Excel is 230 including energy.

### Categories (12)

| SV | EN | n |
| --- | --- | --- |
| Betong | Concrete | 52 |
| Isolering | Insulation | 32 |
| Fönster, dörrar och glas | Windows, doors and glass | 28 |
| Murblock och tegel | Blocks and tiles | 26 |
| Energi och bränsle | Energy and fuel | 22 |
| Byggskivor | Building boards | 18 |
| Stål och andra metaller | Steel and other metals | 17 |
| Bruk och bindemedel | Mineral materials | 16 |
| Tätskikt | Waterproofing | 7 |
| Trävaror | Solid woods | 6 |
| Färg och fog | Paints and sealants | 5 |
| Återbrukade byggprodukter | Reused building products | 1 |

Every row has a category.

### Declared units

| Unit | n |
| --- | --- |
| `kg CO₂e/kg` | 203 |
| `kg CO₂e/MJ` | 20 |
| `kg CO₂e/m²` | 5 (photovoltaic cells) |
| `kg CO₂e/kWh` | 2 (Swedish electricity mix, district heating) |

Energy/fuel is MJ or kWh. Products are kg except five PV modules in m². klimatsearch’s `declaredUnit()` already strips to `kg` / `MJ` / `m²` / `kWh`.

### Conversions

210/230 rows have a conversion factor. Units: 161 `kg/m³`, 32 `kg/m²`, 12 `MJ/liter`, 5 `MJ/kg`. The 20 without conversion include electricity, district heating, some loose-fill insulation, PV modules, reused product, and several fuels. No synonym list lives here.

### Length statistics (characters, nonempty)

| Field | n nonempty | empty | median | p90 | max |
| --- | --- | --- | --- | --- | --- |
| Name SV | 230 | 0 | 30 | 49 | 59 |
| Name EN | 230 | 0 | 36 | 58 | 64 |
| Description SV | 222 | 8 | 188 | 436 | 718 |
| Description EN | 222 | 8 | 200 | 475 | 802 |
| Category SV | 230 | 0 | 11 | 24 | 25 |
| Category EN | 230 | 0 | 15 | 24 | 24 |
| Current `EmbeddingText()` (names+descs, space join) | 230 | 0 | 393 | 971 | 1566 |

Empty descriptions (same 8 IDs in SV and EN): district heating, reused building product, and six fuels (`Gasol`, `Alkylatbensin MK1`, `Naturgas`, `Stamvedsflis`, `Skogsflis`, `Övrigt biobränsle`). For those rows the vector is names only today.

All of this is well under F2LLM-v2-80M’s training max length of 512 tokens (see next section). 1566 characters is roughly a few hundred tokens.

### Uniqueness vs near-duplicates

Exact names are unique: 230/230 SV, 230/230 EN. Unique `(name_sv, name_en)` pairs: 230. Unique current `EmbeddingText()`: 230.

Near-duplicates are the real issue:

- **22** rows share the two-word prefix `Fabriksbetong, husbyggnad` / `Ready-mix made concrete, buildings`, differing by strength class (`C20/25` … `C60/75`) and the token `klimatförbättrad` / `climate-improved`.
- 15 SV name prefixes of length 20 cover 53 rows; 27 EN prefixes cover 87 rows.
- First-word clusters with ≥5 rows: SV `fabriksbetong` 22, `fönster` 10, `stenull` 8, `glasull` 6, `gipsskiva` 6, `solcell` 5.

Descriptions often **do not** separate those siblings. Example (ID `6000000028`): both languages only say the compressive-strength class follows EN 206 / SS 137003 — the same sentence family as other ready-mix grades. Disambiguation lives in the **name**, not the description.

### Bilingual name overlap

| | n |
| --- | --- |
| Case-insensitive identical names | 5 (`Diesel, fossil`, `HVO100`, `OSB`, `FAME100`, `ED95`) |
| Different SV vs EN names | 225 |
| Of those, no shared alphanumeric token (true translations) | 122 |

Examples of true translations: `Spånskiva` / `Particle board`; `Gipsskiva, standardskiva` / `Gypsum, standard plasterboard`; `Fabriksbetong, husbyggnad C25/30` / `Ready-mix made concrete, buildings C25/30`. A Swedish query `spånskiva` will not BM25-match the English name, and an English query `particle board` will not BM25-match `Spånskiva`. FTS is already language-filtered (`{name_sv description_sv}` vs `{name_en description_en}` in `internal/store/store.go`). The vector is the cross-lingual path.

### What klimatsearch embeds today

From `internal/model/resource.go`:

```go
parts := []string{r.NameSV, r.NameEN, r.DescriptionSV, r.DescriptionEN}
// trim empties, join with " "
```

No instruction prefix. No category, unit, A1A3, or conversions. Queries are raw user strings (`internal/search/engine.go`). Hybrid search is FTS ∪ KNN fused with RRF k=60 (ADR-0006). One `float[320]` cosine vector per Resource (`migrations/00001_init.sql`). Production embedder is planned F2LLM-v2-80M; code today uses the fake SHA-256 embedder unless an ONNX file is present (ADR-0002). ONNX inference is not wired (`internal/embedder/onnx.go` returns an error after session init).

---

## F2LLM-v2 usage

Primary sources:

- Model card: <https://huggingface.co/codefuse-ai/F2LLM-v2-80M>
- Paper: Zhang, Liao, Yu, Di, Wang, *F2LLM-v2: Inclusive, Performant, and Efficient Embeddings for a Multilingual World*, arXiv:2603.19223, 19 Mar 2026. <https://arxiv.org/abs/2603.19223> HTML: <https://arxiv.org/html/2603.19223>
- Training code: <https://github.com/codefuse-ai/CodeFuse-Embeddings> (`F2LLM/` tree)
- `config.json`: <https://huggingface.co/codefuse-ai/F2LLM-v2-80M/raw/main/config.json>
- Sentence-Transformers prompts: <https://huggingface.co/codefuse-ai/F2LLM-v2-80M/raw/main/config_sentence_transformers.json>
- Pooling module: <https://huggingface.co/codefuse-ai/F2LLM-v2-80M/raw/main/1_Pooling/config.json>
- Tokenizer: <https://huggingface.co/codefuse-ai/F2LLM-v2-80M/raw/main/tokenizer_config.json>

### Architecture

| | Value | Source |
| --- | --- | --- |
| Family | Instruct model, pruned from F2LLM-v2-0.6B-Preview, itself Qwen3-0.6B | model card model tree; paper §3.2 |
| Params | 80.1M, bf16 | model card |
| `architectures` | `Qwen3Model` | `config.json` |
| `hidden_size` / embedding dim | **320** | `config.json`; encode example prints `(320,)` |
| Layers / heads / KV / head_dim | 8 / 16 / 8 / 128 | `config.json`; paper Table 1 |
| `intermediate_size` | 2048 | `config.json` |
| Pooling | **final hidden state of the EOS token**, then L2-normalize | paper §3.2; model-card Transformers snippet; ST pooling `pooling_mode_lasttoken: true` |
| Similarity | cosine | `config_sentence_transformers.json` `"similarity_fn_name": "cosine"` |
| MRL | yes; smallest trained dim **8**; truncate **then** re-normalize | model card “MRL Support”; paper §3.3; Kusupati et al. 2022 |
| Vocab | 151936, Qwen2Tokenizer | `config.json`, `tokenizer_config.json` |
| EOS / pad | EOS `<|im_end|>` id 151645; pad `<|endoftext|>` id 151643 | `tokenizer_config.json`, `config.json` |
| `max_position_embeddings` | 40960 | `config.json` |
| `model_max_length` | 131072 | `tokenizer_config.json` (inherited Qwen; not the training length) |
| Training max length (80M) | **512** | paper Table 1 (80M column) |
| Tokenizer note | “the tokenizer will automatically add eos token” | model-card Transformers example |

Do not use mean/CLS pooling. Do not skip L2-norm. If you ever truncate to an MRL prefix, normalize **after** truncation (model card).

### Query prefix vs document

From the model card **Prompts** section, verbatim:

> In general, for retrieval and reranking tasks:
>
> - use the prompt for queries
> - do not prepend the prompt to documents/passages
>
> For symmetric tasks such as STS, clustering, and bitext mining, you can encode the documents either with or without prompts.

Default Sentence-Transformers prompts (`config_sentence_transformers.json`):

```json
"query": "Instruct: Given a question, retrieve passages that can help answer the question.\nQuery: ",
"document": ""
```

`encode_query` applies the query prompt; `encode_document` does not. The Transformers example concatenates `query_prompt + query` and encodes documents raw.

Custom instructions are supported in the same shape:

```text
Instruct: your_instruction
Query:
```

Paper §3.3: stage 1 trains on raw retrieval data with **no** instructional prefix; stage 2 applies task-specific instructions **to queries**, and randomly to 30 % of documents only on **symmetric** tasks (clustering, STS, bitext, paraphrase). klimatsearch search is **asymmetric retrieval** (short query → product card). Documents must stay unprefixed.

The tokenizer ships a Qwen3 **chat template**. F2LLM’s official encode path does **not** use it. Applying `apply_chat_template` would be a silent distribution shift.

### Multilingual / Swedish

Paper abstract and §3.1: trained on 60M public samples, **282 natural languages** (ISO-639-3) plus 40+ programming languages, “more than 200 languages”, emphasis on mid- and low-resource. Stage-1 retrieval mix includes ParaCrawl, MMARCO, CLIRMatrix, WebFAQ (cross-lingual / multilingual retrieval).

There is no Swedish-only number. Closest published score is **MTEB Scandinavian (28 tasks)**: F2LLM-v2-80M **55.54** (rank 30 of the leaderboard snapshot on 2026-03-19), Table 2 of the paper. The 14B model is 71.10 (rank 1) on the same board. Treat 80M as “competent multilingual small model”, not as a Scandinavian specialist. Bitext-style SV↔EN matching is in-distribution (ParaCrawl / bitext mining in stage 2); construction jargon is not.

### Recommended retrieval usage (from the owners)

1. Documents: raw text, EOS added by tokenizer, last-token pool, L2-norm, cosine.
2. Queries: `Instruct: …\nQuery: ` + user text, same pooling.
3. `SentenceTransformer.encode_query` / `encode_document` if you stay in Python; in Go, replicate that split.
4. Full 320-d unless you have a storage reason. n=230 is not a storage reason.

---

## sqlite-vec and hybrid search (constraint, not a quality lever)

- vec0 KNN with `distance_metric=cosine`: <https://github.com/asg017/sqlite-vec/blob/main/site/features/knn.md>
- `vec_distance_cosine`: <https://github.com/asg017/sqlite-vec/blob/main/site/api-reference.md>
- Hybrid FTS5 + vec0 + RRF k=60, equal weights: Alex Garcia (sqlite-vec author), 2024-10-02, <https://alexgarcia.xyz/blog/2024/sqlite-vec-hybrid-search/index.html>
- RRF original, k conventionally 60: Cormack, Clarke, Buettcher, *Reciprocal Rank Fusion Outperforms Condorcet and Individual Rank Learning Methods*, SIGIR 2009, <https://doi.org/10.1145/1571941.1572114>

klimatsearch already follows that recipe (ADR-0006). With 230 vectors, vec0 is exhaustive KNN. **ANN recall is not the problem.** Changing RRF k or adding DiskANN will not fix a bad document string.

Garcia’s own motivation for hybrid is exact: keyword search returns what you typed; vectors return paraphrases; RRF boosts items that appear in both. That is the right split for this catalog (exact `C25/30` vs paraphrase `klimatförbättrad betong`).

---

## Options considered

No LCA-specific embedding paper was found that owns a recommendation for Boverket-style generic product cards. The options below are scored against (a) this Excel, (b) F2LLM-v2’s documented retrieval protocol, (c) instruction-tuned embedder practice (E5, E5-Mistral, GTE), and (d) known failure modes of dense retrieval on tiny, peaked corpora.

### 1. Names only (SV+EN)

Pros: shortest, unique, contains the tokens that actually separate ready-mix grades. Cons: no category, so `isolering` / `insulation` has nothing to match if the name is `Stenull, skivor`. Empty-description energy rows are already names-only and are the weakest documents. Cross-lingual still works if both names are present.

Use as an **ablation**, not the production string.

### 2. Names + descriptions (current)

Pros: already implemented; descriptions add manufacturing/use prose (median ~190–200 chars). Cons: unlabeled language concat (`Spånskiva Particle board Spånskivor är gjorda…`); category and unit absent; descriptions often shared across near-duplicates; 8 rows have empty descriptions. Current space-join also glues SV and EN with no boundary, which is worse for last-token pooling than labeled lines (the EOS state has to represent an undifferentiated bag).

This is a reasonable baseline to beat, not the target.

### 3. Names + descriptions + category + unit (recommended)

Category is 12 values, always filled, and is exactly the vocabulary users type (`betong`, `isolering`, `stål`). Unit separates products (kg) from energy (MJ/kWh) and PV (m²). Both fields are short. FTS does **not** index them today, so the vector is the only place they can help until FTS is widened.

Keep descriptions: they help true translations and empty-name-token queries (`particle board`, `blowing wool`). They will not separate `C25/30` vs `C30/37`; FTS and the name string must do that.

### 4. Bilingual concat vs two vectors vs language tags

| Scheme | Storage | Query | Fits klimatsearch? |
| --- | --- | --- | --- |
| Unlabeled concat (current) | 1×320 | 1 embed | yes, but EOS mixes languages |
| **Labeled bilingual concat** | 1×320 | 1 embed | **yes — recommended** |
| Two vectors (SV doc, EN doc) | 2×320 | embed query, search both, fuse | schema change; doubles KNN; FTS already language-splits |
| Language-tagged single field, one language only | 1×320 | must know lang | breaks cross-lingual (`spånskiva` vs `particle board`) |

F2LLM-v2 stage 2 treats bitext as a **symmetric** task (optional prompt on both sides). klimatsearch queries are asymmetric and in **one** language. Putting both names in one document is the standard way to serve both query languages from a single prebuilt index (same argument as E5-Mistral: “We do not modify the document side … the document index can be prebuilt”). Two vectors would need query-time fusion on 460 rows — still tiny, but it changes `resource_vec` and RRF for no measured gain.

Labeled lines (`name_sv:` / `name_en:`) are a cheap structure prior. They are not claimed by F2LLM; they are a document-side formatting choice that does not violate “no instruction prefix on passages”.

### 5. Numeric A1A3 in the document text

Do **not** add `A1A3: 0.39` (or conservative 0.488) to the embedding string.

Reasons, each with an owner:

1. **Policy:** typical vs conservative is a declaration rule, not a retrieval synonym. Boverket forbids typical values in the final climate declaration. Putting one number in the vector invites “low carbon concrete” to rank by embedding proximity to the digit string, not by structured compare. klimatsearch already exposes A1A3 on the Resource and has `/api/resources/compare`.
2. **Numeracy of subword models:** Wallace et al., *Do NLP Models Know Numbers?*, EMNLP 2019, <https://arxiv.org/abs/1909.07940> — BERT-style subword embeddings capture magnitude poorly compared with character models, and probes fail to extrapolate. Qwen3 uses a subword tokenizer (F2LLM `tokenizer_class: Qwen2Tokenizer`). Fine-grained ranking of 0.064 vs 0.39 vs 3.15 is not a job for cosine on 320-d last-token states.
3. **Near-duplicates:** ready-mix grades already encode the distinguishing token in the **name** (`C25/30`). A1A3 values are correlated with grade but are not the user’s query language.
4. **Asymmetric retrieval protocol:** E5 and E5-Mistral put task semantics on the query, not extra document fields of a different type. Adding floats to passages does not make the query “cheapest climate impact” match the right row.

If a later golden set shows users query by magnitude (“betong under 0.2 kg CO2e/kg”), implement it as a **structured filter** on `a1_a3`, not as text in the vector.

### Instruction prefixes (E5 / GTE / F2LLM)

| Model | Query | Document | Source |
| --- | --- | --- | --- |
| E5 | `query: ` | `passage: ` | Wang et al. 2022, arXiv:2212.03533; <https://huggingface.co/intfloat/e5-base-v2> (“Yes, this is how the model is trained, otherwise you will see a performance degradation.”) |
| E5-Mistral / LLM embedders | `Instruct: {task}\nQuery: {q}` | **no prefix** | Wang et al. 2024, arXiv:2401.00368 §3.2 |
| Original GTE | no task prompt | no task prompt | Li et al. 2023, arXiv:2308.03281 (“our model does not incorporate task-specific prompts”) |
| **F2LLM-v2** | `Instruct: …\nQuery: ` | **empty** | model card + ST config |

klimatsearch planned the F2LLM protocol (ADR-0002). Follow F2LLM, not E5’s `query:`/`passage:` pair and not GTE’s no-prompt setup. Putting the F2LLM query prompt on documents would train-test mismatch the stage-2 retrieval format.

A domain-specific `Instruct:` line is explicitly allowed by the model card. Prefer a construction-catalog instruction over the generic “retrieve passages that can help answer the question”, then **measure** both on the golden set.

---

## Recommendation with why

### Document

Labeled bilingual concat of **both names, both categories, declared unit, both descriptions**. One vector. No A1A3. No conversions. No BK04 name (too coarse: 14 mineral-wool products collapse to `Mineralull`). No instruction prefix. No chat template.

This is option 3 plus language tags from option 4.

### Query

F2LLM `Instruct`/`Query` wrapper around the raw user string. Same pooling. Do not language-tag the query (the user already typed one language; FTS uses `lang` separately).

### Dim and pooling

320-d last-token / EOS, L2-norm, sqlite-vec cosine. MRL truncation is unused until a second model or a storage budget appears. Paper Figure 5 shows the steep MRL gains from dim 8→128 and a plateau toward full size; at n=230 the full 320-d vector is free.

### Why this beats the current string

1. **Query–document asymmetry** is how F2LLM-v2, E5-Mistral, and the model card say to run retrieval. Today both sides are raw. That is the largest protocol bug, larger than the document template.
2. **Category** is the missing high-value field: 12 closed values, user vocabulary, not in FTS.
3. **Unit** is the energy-vs-product split.
4. **Labels** give the last-token state a chance to keep SV and EN distinct instead of a 400–1500 character undifferentiated bag.
5. **Near-duplicates** remain a name/FTS problem. The vector should recall the cluster (`fabriksbetong` / `ready-mix`); RRF + BM25 (and later a reranker) should order `C25/30` vs `C30/37`. That is exactly Garcia’s hybrid argument and ADR-0006.

### Tiny-corpus effects (why ANN does not matter)

- **Hubness.** Radovanović, Nanopoulos, Ivanović, *Hubs in Space: Popular Nearest Neighbors in High-Dimensional Data*, JMLR 11:2487–2531, 2010, <https://www.jmlr.org/papers/v11/radovanovic10a.html>. In high-d cosine spaces some points become neighbors of many queries. 320-d with 230 peaked concrete/wool/window vectors is a hubness setting, not an IVF/HNSW setting. Mitigation: hybrid RRF (already on), exact names in FTS (already on), and not smearing 22 ready-mix rows into identical description-dominated vectors.
- **Near-duplicate names.** Cosine will not stably order `C25/30` vs `klimatförbättrad C25/30` if descriptions are identical. Do not evaluate the embedder on that distinction; evaluate FTS+RRF.
- **Query–document asymmetry.** Short queries vs 200–800 char cards is the E5/F2LLM retrieval setting. Symmetric STS-style encoding (prompt on both sides) is wrong here (F2LLM card).
- **Fake embedder.** `internal/embedder/fake.go` is SHA-256 of UTF-8 expanded to 320 float32 and L2-normalized. Near-duplicate names hash to unrelated points. Any Recall@k on Fake is a test of string identity, not semantics. Do not tune the document template against it.

### What to measure on a tiny golden set (10–20 queries, SV and EN)

Build the set by hand from the Excel, not from the fake embedder. Suggested pairs (adjust IDs after a second look at the sheet):

| Intent | Example SV | Example EN | Expect |
| --- | --- | --- | --- |
| Exact name | `Spånskiva` | `Particle board` | ID `6000000000` @1 |
| True translation | `gipsskiva standard` | `gypsum plasterboard` | standard gypsum, not fire/wet-room |
| Near-dup disambiguation | `fabriksbetong C25/30` | `ready-mix C25/30` | the non-improved C25/30 above C28/35 |
| Climate-improved sibling | `klimatförbättrad C30/37` | `climate-improved C30/37` | the improved row, not the plain grade |
| Category | `isolering` | `insulation` | stone/glass wool cluster in top 10 |
| Energy vs product | `svensk elmix` | `Swedish electricity mix` | `6000000008`, not a fuel |
| Empty description | `HVO100` | `HVO100` | exact ID |
| Unit-sensitive | `solcell` | `photovoltaic` | PV rows (m²), not “cells” in other senses |
| Informal paraphrase | `betong till hus` | `ready mix for buildings` | ready-mix cluster |
| Strength class only | `C40/50` | `C40/50` | the two C40/50 rows (plain + improved) |
| Reused product | `återanvänd` | `reused building product` | `6000000174` (empty description) |
| Steel scrap vs primary | `armeringsstål skrot` | `rebar scrap-based` | scrap rebar, not stainless |

Metrics, **with F2LLM-v2-80M ONNX**, not Fake:

- Recall@1 and Recall@5 per query, SV and EN separately (cross-lingual: SV query vs bilingual index, EN query vs bilingual index).
- nDCG@10 on the near-dup and category queries.
- Cluster recall: for a category query, fraction of the gold category in the top 10.
- Sibling inversion count: how often the wrong ready-mix grade outranks the right one.
- Ablations on the same queries: names-only; names+descs (current); names+descs+cat+unit unlabeled; **labeled recommended**; recommended plus A1A3 (expect no gain or harm).
- Prefix ablations: no prefix; model-card default Instruct; domain Instruct.

Do not report a winner until the real model runs. n=230 means you can re-embed the whole catalog in seconds once ONNX works.

Optional FTS experiment (out of embedding scope, but it is the other half of RRF): add `category` (and maybe unit) to `resources_fts`. That is likely higher leverage for `isolering` than any vector trick.

---

## Open risks

### ONNX export of Qwen3 80M

`scripts/download-models.sh` downloads Hugging Face weights and comments an Optimum example:

```python
from optimum.onnxruntime import ORTModelForFeatureExtraction
model = ORTModelForFeatureExtraction.from_pretrained("codefuse-ai/F2LLM-v2-80M", export=True)
```

Risks that the script does not own:

- F2LLM-v2-80M is a **decoder-only Qwen3** (`architectures: Qwen3Model`), not a BERT encoder. `ORTModelForFeatureExtraction` may export last hidden states for all tokens, not the EOS vector. Go must gather `last_hidden[arange, eos_positions]` using `attention_mask.sum(1)-1`, matching the model-card snippet.
- Weights are **bf16**. ONNX Runtime on macOS/CPU often wants float32. Export dtype is an untested choice.
- `transformers>=4.51.0` is required for Qwen3 (F2LLM README; Qwen3-0.6B card: `KeyError: 'qwen3'` below that). Optimum/ORT version skew is a real export failure mode.
- Dynamic sequence length vs fixed 512: training max is 512; config allows 40960. Export a dynamic axis or 512; klimatsearch docs are all ≪512 tokens.

klimatsearch `ONNX.Embed` currently initializes a session and then errors out (“inference is not run without a wired tokenizer”). Export success ≠ retrieval quality.

### Tokenizer

- Official path: Hugging Face `tokenizer.json` (Qwen2Tokenizer / Qwen3). EOS must be appended (“tokenizer will automatically add eos token”).
- Go: `FileTokenizer` is a stub that errors even when the file exists (`internal/embedder/tokenizer.go`). Need a real Hugging Face tokenizer (tokenizers CGO, a WASM build, or pre-tokenize in a sidecar — all out of scope here).
- **Do not** apply the shipped chat template. **Do** add EOS. **Do** pad with `<|endoftext|>` if you batch.
- Last-token pooling is wrong if EOS is missing: you would pool a pad or the last content token, which is not how the model was trained.

### Instruction prefix in Go

- `Engine.Search` embeds `query` raw. Ingest embeds `EmbeddingText()` raw. Both need the F2LLM split: prefix on search queries only.
- Prefix text must be **byte-identical** to training (`Instruct: …\nQuery: ` with that newline). A missing newline is a different prompt.
- Domain vs default Instruct should be a single constant, tested on the golden set, not inferred per query.
- Reranker (when added) is a different model; do not assume it wants the same prefix.

### Other

- Changing `EmbeddingText()` changes `ContentHash()` only if those fields are already in the hash (category, unit, names, descriptions already are; formatting of the embedding string is **not** in the hash). Re-embed will not happen on format-only changes unless you bump a version or include the embedding template in the hash. Plan an explicit re-embed.
- `meta.embedding_dim=320` must stay 320 (ADR-0002). MRL-truncated vectors are a different dim and would disable KNN via `ConfigureVector`.
- Fake embedder tests will not catch prefix or pooling bugs. Add a fixture that records F2LLM vectors only in non-CI, or snapshot a few Python-encoded gold vectors.

---

## Sources (all claims above trace here)

**Boverket**

- Climate database (EN): <https://www.boverket.se/en/start/laws-and-regulations/climate-declaration/climate-database/>
- Open data (SV): <https://www.boverket.se/sv/om-boverket/oppna-data/boverkets-klimatdatabas/>
- Version 02.07.000 news: <https://www.boverket.se/sv/klimatdeklaration/om-klimatdeklaration/nyheter/ny-version-klimatdatabas/>
- Excel SV: <https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-klimatdatabas-version-02.07.000-sv-se.xlsx>
- Excel EN: <https://www.boverket.se/contentassets/4668ed4cc3da447385788ed30bff7d49/boverkets-climate-database-version-02.07.000-en-gb.xlsx>
- klimatsearch ADR-0005 (typical A1–A3, Excel fallback URLs): `docs/adr/0005-boverket-json-api-with-excel-fallback.md`

**F2LLM-v2**

- <https://huggingface.co/codefuse-ai/F2LLM-v2-80M>
- <https://arxiv.org/abs/2603.19223>
- <https://github.com/codefuse-ai/CodeFuse-Embeddings>
- Config / pooling / prompts / tokenizer raw files linked in the F2LLM section

**Instruction-tuned embedders**

- E5: Wang et al. 2022, <https://arxiv.org/abs/2212.03533>, card <https://huggingface.co/intfloat/e5-base-v2>
- E5-Mistral / LLM instructions: Wang et al. 2024, <https://arxiv.org/abs/2401.00368>
- GTE: Li et al. 2023, <https://arxiv.org/abs/2308.03281>
- MRL: Kusupati et al. 2022, <https://arxiv.org/abs/2205.13147>
- Qwen3 (backbone, chat template, transformers≥4.51): <https://huggingface.co/Qwen/Qwen3-0.6B>, <https://arxiv.org/abs/2505.09388>

**Retrieval / hybrid / failure modes**

- sqlite-vec KNN: <https://github.com/asg017/sqlite-vec/blob/main/site/features/knn.md>
- Garcia hybrid FTS5+vec0+RRF: <https://alexgarcia.xyz/blog/2024/sqlite-vec-hybrid-search/index.html>
- RRF: Cormack et al., SIGIR 2009, <https://doi.org/10.1145/1571941.1572114>
- Hubness: Radovanović et al., JMLR 2010, <https://www.jmlr.org/papers/v11/radovanovic10a.html>
- Numeracy: Wallace et al., EMNLP 2019, <https://arxiv.org/abs/1909.07940>

**klimatsearch (current behavior)**

- `internal/model/resource.go` (`EmbeddingText`)
- `internal/search/engine.go` (raw query embed, RRF k=60)
- `internal/embedder/fake.go`, `onnx.go`, `tokenizer.go`
- `internal/store/store.go` (language-scoped FTS, cosine vec0)
- `migrations/00001_init.sql` (`embedding float[320] distance_metric=cosine`)
- ADR-0002, ADR-0006, `docs/prd.md`
- `scripts/download-models.sh`
