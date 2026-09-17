package search

import (
	"context"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Hit is one Resource returned for a Query, including the stored row.
type Hit struct {
	ID              string
	CatalogID       string
	NameSV          string
	NameEN          string
	DescriptionSV   string
	DescriptionEN   string
	ApplicabilitySV string
	ApplicabilityEN string
	Synonyms        string
	A1A3            float64
	Unit            string
	Conversions     map[string]float64
	Category        string
	CategoryCode    string
	Version         string
	Lang            string
	Score           float64
	Source          string // "fts" | "vector" | "both" | "rerank"
	Details         string // GET path for this Resource
	Origin          string // Catalog's own page, if known
}

func HitFrom(r model.Resource, lang, source string, score float64) Hit {
	conv := r.Conversions
	if conv == nil {
		conv = map[string]float64{}
	} else {
		cp := make(map[string]float64, len(conv))
		for k, v := range conv {
			cp[k] = v
		}
		conv = cp
	}
	return Hit{
		ID:              r.ResourceID,
		CatalogID:       r.CatalogID,
		NameSV:          r.NameSV,
		NameEN:          r.NameEN,
		DescriptionSV:   r.DescriptionSV,
		DescriptionEN:   r.DescriptionEN,
		ApplicabilitySV: r.ApplicabilitySV,
		ApplicabilityEN: r.ApplicabilityEN,
		Synonyms:        r.Synonyms,
		A1A3:            r.A1A3,
		Unit:            r.Unit,
		Conversions:     conv,
		Category:        r.Category,
		CategoryCode:    r.CategoryCode,
		Version:         r.Version,
		Lang:            lang,
		Score:           score,
		Source:          source,
		Details:         "/api/resources/" + r.DocID(),
		Origin:          r.Origin(),
	}
}

// View is the search-hit JSON: full Resource plus score, match_source, details.
func (h Hit) View(attribution string) map[string]any {
	r := model.Resource{
		CatalogID:       h.CatalogID,
		ResourceID:      h.ID,
		NameSV:          h.NameSV,
		NameEN:          h.NameEN,
		DescriptionSV:   h.DescriptionSV,
		DescriptionEN:   h.DescriptionEN,
		ApplicabilitySV: h.ApplicabilitySV,
		ApplicabilityEN: h.ApplicabilityEN,
		Synonyms:        h.Synonyms,
		A1A3:            h.A1A3,
		Unit:            h.Unit,
		Conversions:     h.Conversions,
		Category:        h.Category,
		CategoryCode:    h.CategoryCode,
		Version:         h.Version,
	}
	m := r.View(attribution)
	m["lang"] = h.Lang
	m["score"] = h.Score
	m["match_source"] = h.Source
	m["details"] = h.Details
	if h.Origin != "" {
		m["origin"] = h.Origin
	}
	return m
}

// Embedder turns document text into a fixed-dimension vector.
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// QueryEmbedder embeds a Query. F2LLM applies an Instruct prefix on Queries only.
// Fake embedders omit this method so tests hash the raw Query.
type QueryEmbedder interface {
	EmbedQuery(query string) ([]float32, error)
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
