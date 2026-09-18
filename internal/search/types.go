package search

import (
	"context"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Query is a request for Hits from Klimatdatabas.
type Query struct {
	Text     string
	Lang     string
	Catalogs []string
	Vector   bool
	Rerank   bool
}

// Hit is a Resource returned for a Query, plus retrieval fields.
type Hit struct {
	model.Resource
	Lang   string
	Score  float64
	Source string // "fts" | "vector" | "both" | "rerank"
}

func HitFrom(r model.Resource, lang, source string, score float64) Hit {
	return Hit{Resource: r, Lang: lang, Score: score, Source: source}
}

// View is Resource JSON plus score and match_source.
// REST stamps details (/api/resources/…) on the Envelope; MCP does not.
func (h Hit) View(attribution string) map[string]any {
	m := h.Resource.View(attribution)
	m["lang"] = h.Lang
	m["score"] = h.Score
	m["match_source"] = h.Source
	return m
}

// Envelope is the shared search JSON for REST and MCP.
func Envelope(attribution string, q Query, hits []Hit) map[string]any {
	if hits == nil {
		hits = []Hit{}
	}
	results := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		results = append(results, h.View(attribution))
	}
	return map[string]any{
		"source":  attribution,
		"query":   q.Text,
		"lang":    q.Lang,
		"results": results,
	}
}

// Embedder maps Resource text or a Query to a vector.
// EmbedQuery is the Query path (F2LLM Instruct on ONNX; Fake hashes the raw Query).
type Embedder interface {
	Embed(text string) ([]float32, error)
	EmbedQuery(query string) ([]float32, error)
}

// Reranker reorders Hits after retrieval.
type Reranker interface {
	Rerank(query string, docs []Hit) ([]Hit, error)
}

// SearchEngine is hybrid retrieval over Klimatdatabas.
type SearchEngine interface {
	Search(ctx context.Context, q Query) ([]Hit, error)
}

// Dimensional is implemented by embedders that know their output width.
type Dimensional interface {
	Dim() int
}
