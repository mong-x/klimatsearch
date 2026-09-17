package search

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// LangSV and LangEN are the only accepted language codes.
const (
	LangSV      = "sv"
	LangEN      = "en"
	DefaultLang = LangSV
)

// ErrBadLang is returned when lang is not sv or en.
type ErrBadLang struct{ Lang string }

func (e ErrBadLang) Error() string {
	return fmt.Sprintf("unsupported lang %q (sv|en)", e.Lang)
}

// NormalizeLang returns sv or en, defaulting empty to sv.
func NormalizeLang(lang string) (string, error) {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		return DefaultLang, nil
	}
	if lang != LangSV && lang != LangEN {
		return "", ErrBadLang{Lang: lang}
	}
	return lang, nil
}

// Engine is a hybrid FTS + vector + optional rerank SearchEngine.
type Engine struct {
	fts      FTS
	vec      Vector
	embedder Embedder
	reranker Reranker
	log      *slog.Logger
	limit    int
}

// FTS is BM25 search. Store implements this and returns Ranked, not Hits.
type FTS interface {
	SearchFTS(ctx context.Context, query, lang string, limit int, catalogs []string) ([]model.Ranked, error)
}

// Vector is KNN search.
type Vector interface {
	SearchVector(ctx context.Context, embedding []float32, lang string, limit int, catalogs []string) ([]model.Ranked, error)
	VectorEnabled() bool
	VectorSkipReason() string
}

func New(fts FTS, vec Vector, emb Embedder, rr Reranker) *Engine {
	if rr == nil {
		rr = nopReranker{}
	}
	return &Engine{fts: fts, vec: vec, embedder: emb, reranker: rr, log: slog.Default(), limit: 20}
}

type nopReranker struct{}

func (nopReranker) Rerank(_ string, docs []Hit) ([]Hit, error) {
	return docs, nil
}

func (e *Engine) Search(ctx context.Context, query string, useVector bool, useRerank bool, lang string, catalogs []string) ([]Hit, error) {
	lang, err := NormalizeLang(lang)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	ftsRanked, err := e.fts.SearchFTS(ctx, query, lang, e.limit, catalogs)
	if err != nil {
		return nil, err
	}

	var vecRanked []model.Ranked
	if useVector {
		if e.vec == nil || !e.vec.VectorEnabled() {
			reason := "vector backend unavailable"
			if e.vec != nil {
				reason = e.vec.VectorSkipReason()
			}
			e.log.Warn("skipping vector search", "reason", reason)
		} else if e.embedder == nil {
			e.log.Warn("skipping vector search", "reason", "no embedder")
		} else {
			emb, err := embedQuery(e.embedder, query)
			if err != nil {
				return nil, fmt.Errorf("embed query: %w", err)
			}
			vecRanked, err = e.vec.SearchVector(ctx, emb, lang, e.limit, catalogs)
			if err != nil {
				return nil, err
			}
		}
	}

	var merged []Hit
	if useVector && len(vecRanked) > 0 {
		merged = rrf(ftsRanked, vecRanked, lang, e.limit)
	} else {
		merged = ftsHits(ftsRanked, lang)
	}
	if useRerank && e.reranker != nil && len(merged) > 0 {
		merged, err = e.reranker.Rerank(query, merged)
		if err != nil {
			return nil, fmt.Errorf("rerank: %w", err)
		}
		for i := range merged {
			merged[i].Source = "rerank"
			merged[i].Lang = lang
		}
	}
	return merged, nil
}

const rrfK = 60

func ftsHits(fts []model.Ranked, lang string) []Hit {
	out := make([]Hit, 0, len(fts))
	for _, r := range fts {
		out = append(out, HitFrom(r.Resource, lang, "fts", 1.0/(rrfK+float64(r.Rank))))
	}
	return out
}

// rrf fuses FTS and KNN with Reciprocal Rank Fusion, k=60, equal weights.
// Missing list contributes 0. BM25 and cosine are not mixed as raw scores.
func rrf(fts, knn []model.Ranked, lang string, limit int) []Hit {
	type acc struct {
		res   model.Resource
		score float64
		fromF bool
		fromV bool
	}
	byID := map[string]*acc{}
	add := func(list []model.Ranked, fromV bool) {
		for _, r := range list {
			id := r.Resource.DocID()
			s, ok := byID[id]
			if !ok {
				s = &acc{res: r.Resource}
				byID[id] = s
			}
			s.score += 1.0 / (rrfK + float64(r.Rank))
			if fromV {
				s.fromV = true
			} else {
				s.fromF = true
			}
		}
	}
	add(fts, false)
	add(knn, true)
	out := make([]Hit, 0, len(byID))
	for _, s := range byID {
		src := "fts"
		switch {
		case s.fromF && s.fromV:
			src = "both"
		case s.fromV:
			src = "vector"
		}
		out = append(out, HitFrom(s.res, lang, src, s.score))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].CatalogID != out[j].CatalogID {
			return out[i].CatalogID < out[j].CatalogID
		}
		return out[i].ID < out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func embedQuery(e Embedder, query string) ([]float32, error) {
	if qe, ok := e.(QueryEmbedder); ok {
		return qe.EmbedQuery(query)
	}
	return e.Embed(query)
}
