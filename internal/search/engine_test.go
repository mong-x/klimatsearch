package search_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/hash"
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
	h, _ := hash.Content(r)
	r.ContentHash = h
	vec, _ := fake.Embed(r.EmbeddingText())
	if err := st.Upsert(t.Context(), r, vec); err != nil {
		t.Fatal(err)
	}
	eng := search.New(st, st, fake, reranker.None{})

	hits, err := eng.Search("Betong", false, false, "sv")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != r.ResourceID {
		t.Fatalf("fts: %+v", hits)
	}

	hits, err = eng.Search(r.EmbeddingText(), true, false, "sv")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != r.ResourceID {
		t.Fatalf("vector: %+v", hits)
	}

	if _, err := eng.Search("x", false, false, "xx"); err == nil {
		t.Fatal("expected bad lang")
	}
}
