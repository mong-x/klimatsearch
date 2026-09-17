package search

import (
	"context"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Hit is one Resource returned for a Query.
type Hit struct {
	ID            string
	CatalogID     string
	NameSV        string
	NameEN        string
	DescriptionSV string
	DescriptionEN string
	A1A3          float64
	Unit          string
	Lang          string
	Score         float64
	Source        string // "fts" | "vector" | "both" | "rerank"
}

func HitFrom(r model.Resource, lang, source string, score float64) Hit {
	return Hit{
		ID:            r.ResourceID,
		CatalogID:     r.CatalogID,
		NameSV:        r.NameSV,
		NameEN:        r.NameEN,
		DescriptionSV: r.DescriptionSV,
		DescriptionEN: r.DescriptionEN,
		A1A3:          r.A1A3,
		Unit:          r.Unit,
		Lang:          lang,
		Score:         score,
		Source:        source,
	}
}

// Embedder turns text into a fixed-dimension vector.
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// Reranker reorders Hits after retrieval.
type Reranker interface {
	Rerank(query string, docs []Hit) ([]Hit, error)
}

// SearchEngine is hybrid retrieval over Klimatdatabas.
type SearchEngine interface {
	Search(ctx context.Context, query string, useVector bool, useRerank bool, lang string, catalogs []string) ([]Hit, error)
}

// Dimensional is implemented by embedders that know their output width.
type Dimensional interface {
	Dim() int
}
