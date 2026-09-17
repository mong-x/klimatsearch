package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
)

func TestOpenRequiresCGO(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	st.ConfigureVector(embedder.Dim)
	return st
}

func TestGetByIDAndList(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	r := fixtureBetong()
	h, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.Hash = h
	var f embedder.Fake
	vec, err := f.Embed(r.EmbeddingText())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(ctx, r, vec); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(ctx, r.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NameSV != "Betong" || got.NameEN != "Concrete" {
		t.Fatalf("got %+v", got)
	}
	list, err := st.List(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list %d", len(list))
	}
	if list[0].CatalogID != "boverket" {
		t.Fatalf("catalog=%s", list[0].CatalogID)
	}
	gotPrefixed, err := st.Get(ctx, "boverket:"+r.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	if gotPrefixed.NameSV != "Betong" {
		t.Fatalf("prefixed get %+v", gotPrefixed)
	}
	_, err = st.Get(ctx, "missing")
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestFTSSwedishName(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	seed(t, st, fixtureBetong(), fixtureSteel())
	hits, err := st.SearchFTS(ctx, "Betong", "sv", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected FTS hit for Betong")
	}
	if hits[0].Resource.ResourceID != "6000000991" || hits[0].Rank != 1 {
		t.Fatalf("want betong id rank 1, got %+v", hits)
	}
}

func TestVectorExactText(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	r := fixtureBetong()
	seed(t, st, r, fixtureSteel())
	var f embedder.Fake
	q, err := f.Embed(r.EmbeddingText())
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.SearchVector(ctx, q, "sv", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected vector hit")
	}
	if hits[0].Resource.ResourceID != r.ResourceID || hits[0].Rank != 1 {
		t.Fatalf("want %s first rank 1, got %+v", r.ResourceID, hits)
	}
}

func TestDimMismatchDisablesVector(t *testing.T) {
	st := testStore(t)
	st.ConfigureVector(160)
	if st.VectorEnabled() {
		t.Fatal("vector should be disabled on dim mismatch")
	}
}

func seed(t *testing.T, st *Store, rs ...model.Resource) {
	t.Helper()
	ctx := context.Background()
	var f embedder.Fake
	for _, r := range rs {
		h, err := r.ContentHash()
		if err != nil {
			t.Fatal(err)
		}
		r.Hash = h
		vec, err := f.Embed(r.EmbeddingText())
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Upsert(ctx, r, vec); err != nil {
			t.Fatal(err)
		}
	}
}

func fixtureBetong() model.Resource {
	return model.Resource{
		ResourceID:    "6000000991",
		NameSV:        "Betong",
		NameEN:        "Concrete",
		DescriptionSV: "Generisk betong för stomme och grund.",
		DescriptionEN: "Generic concrete for structure and foundations.",
		A1A3:          0.12,
		Unit:          "kg",
		Conversions:   map[string]float64{"kg/m³": 2400},
		Category:      "Betong",
		Version:       "02.07.000-fixture",
	}
}

func TestAmbiguousGetAndCatalogFilter(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	a := fixtureBetong()
	a.CatalogID = model.CatalogBoverket
	b := fixtureBetong()
	b.CatalogID = model.CatalogBR25
	b.NameSV = "BR25 Betong"
	seed(t, st, a, b)

	_, err := st.Get(ctx, a.ResourceID)
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("want ErrAmbiguous, got %v", err)
	}
	got, err := st.Get(ctx, "br25:"+a.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	if got.NameSV != "BR25 Betong" {
		t.Fatalf("%+v", got)
	}

	onlyBR, err := st.List(ctx, []string{model.CatalogBR25})
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBR) != 1 || onlyBR[0].CatalogID != model.CatalogBR25 {
		t.Fatalf("%+v", onlyBR)
	}

	hits, err := st.SearchFTS(ctx, "Betong", "sv", 10, []string{model.CatalogBR25})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Resource.CatalogID != model.CatalogBR25 {
		t.Fatalf("fts filter %+v", hits)
	}
}

func fixtureSteel() model.Resource {
	return model.Resource{
		ResourceID:    "6000000992",
		NameSV:        "Konstruktionsstål",
		NameEN:        "Structural steel",
		DescriptionSV: "Stål för bärande konstruktioner.",
		DescriptionEN: "Steel for load-bearing structures.",
		A1A3:          1.55,
		Unit:          "kg",
		Conversions:   map[string]float64{"kg": 1},
		Category:      "Stål",
		Version:       "02.07.000-fixture",
	}
}
