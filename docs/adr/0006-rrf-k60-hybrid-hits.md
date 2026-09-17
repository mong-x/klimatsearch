# RRF k=60 for hybrid Hits

Hybrid retrieval fuses FTS5 BM25 ranks and sqlite-vec cosine KNN with Reciprocal Rank Fusion, `score = 1/(60+rank_fts) + 1/(60+rank_knn)` (a missing list contributes 0), because BM25 and cosine are incomparable scales and we have no labeled Queries to tune a convex combination or DBSF; sqlite-vec's hybrid recipe is RRF, equal weights, k=60. Hits record how they matched as `fts`, `vector`, or `both`; a Reranker then overwrites Source to `rerank`. Do not linear-mix raw scores — the next quality step is the Reranker.
