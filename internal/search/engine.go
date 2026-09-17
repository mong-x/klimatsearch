package search

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

// FTS is BM25 search.
type FTS interface {
	SearchFTS(ctx context.Context, query, lang string, limit int) ([]SearchResult, error)
}

// Vector is KNN search.
type Vector interface {
	SearchVector(ctx context.Context, embedding []float32, lang string, limit int) ([]SearchResult, error)
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

func (nopReranker) Rerank(_ string, docs []SearchResult) ([]SearchResult, error) {
	return docs, nil
}

func (e *Engine) Search(query string, useVector bool, useRerank bool, lang string) ([]SearchResult, error) {
	return e.SearchContext(context.Background(), query, useVector, useRerank, lang)
}

func (e *Engine) SearchContext(ctx context.Context, query string, useVector bool, useRerank bool, lang string) ([]SearchResult, error) {
	lang, err := NormalizeLang(lang)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	ftsHits, err := e.fts.SearchFTS(ctx, query, lang, e.limit)
	if err != nil {
		return nil, err
	}

	var vecHits []SearchResult
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
			emb, err := e.embedder.Embed(query)
			if err != nil {
				return nil, fmt.Errorf("embed query: %w", err)
			}
			vecHits, err = e.vec.SearchVector(ctx, emb, lang, e.limit)
			if err != nil {
				return nil, err
			}
		}
	}

	merged := merge(ftsHits, vecHits, e.limit)
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

func merge(fts, vec []SearchResult, limit int) []SearchResult {
	type scored struct {
		res   SearchResult
		best  float64
		fromV bool
	}
	order := make([]string, 0, len(fts)+len(vec))
	byID := map[string]*scored{}
	add := func(r SearchResult, fromV bool) {
		s, ok := byID[r.ID]
		if !ok {
			cp := r
			byID[r.ID] = &scored{res: cp, best: r.Score, fromV: fromV}
			order = append(order, r.ID)
			return
		}
		if r.Score > s.best {
			s.best = r.Score
			s.res.Score = r.Score
		}
		if fromV {
			s.fromV = true
			s.res.Source = "vector"
		}
	}
	for _, r := range fts {
		add(r, false)
	}
	for _, r := range vec {
		add(r, true)
	}
	out := make([]SearchResult, 0, len(order))
	for _, id := range order {
		s := byID[id]
		s.res.Score = s.best
		if s.fromV {
			s.res.Source = "vector"
		}
		out = append(out, s.res)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
