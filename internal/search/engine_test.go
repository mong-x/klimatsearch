package search_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

func TestNormalizeLang(t *testing.T) {
	got, err := search.NormalizeLang("")
	if err != nil || got != "sv" {
		t.Fatalf("default: %q %v", got, err)
	}
	if _, err := search.NormalizeLang("de"); err == nil {
		t.Fatal("expected error for de")
	}
}

func TestEngineFTSAndVector(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	r := model.Resource{
		ResourceID: "6000000991",
		NameSV:     "Betong",
		NameEN:     "Concrete",
		A1A3:       0.12,
		Unit:       "kg",
		Version:    "t",
	}
	h, _ := r.ContentHash()
	r.Hash = h
	vec, _ := fake.Embed(r.EmbeddingText())
	if err := st.Upsert(t.Context(), r, vec); err != nil {
		t.Fatal(err)
	}
	eng := search.New(st, st, fake, reranker.None{})

	hits, err := eng.Search(t.Context(), search.Query{Text: "Betong", Lang: "sv"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ResourceID != r.ResourceID {
		t.Fatalf("fts: %+v", hits)
	}

	hits, err = eng.Search(t.Context(), search.Query{Text: r.EmbeddingText(), Lang: "sv", Vector: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ResourceID != r.ResourceID {
		t.Fatalf("vector: %+v", hits)
	}

	if _, err := eng.Search(t.Context(), search.Query{Text: "x", Lang: "xx"}); err == nil {
		t.Fatal("expected bad lang")
	}
}

func TestEngineRRF(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "rrf.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())

	both := model.Resource{ResourceID: "both", NameSV: "Betong", NameEN: "Concrete", A1A3: 0.12, Unit: "kg"}
	vecOnly := model.Resource{ResourceID: "vec", NameSV: "Konstruktionsstål", NameEN: "Structural steel", A1A3: 1.55, Unit: "kg"}
	ftsOnly := model.Resource{ResourceID: "fts", NameSV: "Betong extra", NameEN: "Extra concrete", A1A3: 0.2, Unit: "kg"}
	for _, r := range []model.Resource{both, vecOnly} {
		h, _ := r.ContentHash()
		r.Hash = h
		vec, _ := fake.Embed(r.EmbeddingText())
		if err := st.Upsert(t.Context(), r, vec); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := ftsOnly.ContentHash()
	ftsOnly.Hash = h
	if err := st.Upsert(t.Context(), ftsOnly, nil); err != nil {
		t.Fatal(err)
	}

	eng := search.New(st, st, fake, reranker.None{})

	ftsHits, err := eng.Search(t.Context(), search.Query{Text: "Betong", Lang: "sv"})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range ftsHits {
		if hit.Source != "fts" {
			t.Fatalf("fts-only search source=%s id=%s", hit.Source, hit.ResourceID)
		}
		if hit.ResourceID == "vec" {
			t.Fatal("vector-only resource should not appear in FTS search")
		}
	}

	hits, err := eng.Search(t.Context(), search.Query{Text: "Betong", Lang: "sv", Vector: true})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]search.Hit{}
	for _, hit := range hits {
		byID[hit.ResourceID] = hit
	}
	b, ok := byID["both"]
	if !ok || b.Source != "both" {
		t.Fatalf("both: %+v", b)
	}
	v, ok := byID["vec"]
	if !ok || v.Source != "vector" {
		t.Fatalf("vec: %+v hits=%+v", v, hits)
	}
	f, ok := byID["fts"]
	if !ok || f.Source != "fts" {
		t.Fatalf("fts: %+v hits=%+v", f, hits)
	}
	if !(b.Score > v.Score && b.Score > f.Score) {
		t.Fatalf("both RRF %v should beat vec %v and fts %v", b.Score, v.Score, f.Score)
	}

	reranked, err := eng.Search(t.Context(), search.Query{Text: "Betong", Lang: "sv", Vector: true, Rerank: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(reranked) == 0 {
		t.Fatal("empty rerank")
	}
	// None is passthrough: retrieval Source stays, Reranked is false.
	for _, hit := range reranked {
		if hit.Source == "rerank" {
			t.Fatalf("retrieval source wiped: %s", hit.Source)
		}
		if hit.Reranked {
			t.Fatal("None must not stamp Reranked")
		}
	}
}
