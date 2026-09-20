package store

import (
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
)

func TestInspectCoverageAndEmbedding(t *testing.T) {
	st := testStore(t)
	ctx := t.Context()
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	withVec := model.Resource{CatalogID: model.CatalogBoverket, ResourceID: "1", NameSV: "Betong", A1A3: 0.1, Unit: "kg"}
	h, _ := withVec.ContentHash()
	withVec.Hash = h
	vec, err := fake.Embed(withVec.EmbeddingText())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(ctx, withVec, vec); err != nil {
		t.Fatal(err)
	}
	noVec := model.Resource{CatalogID: model.CatalogBoverket, ResourceID: "2", NameSV: "Stål", A1A3: 1.2, Unit: "kg"}
	h2, _ := noVec.ContentHash()
	noVec.Hash = h2
	if err := st.Upsert(ctx, noVec, nil); err != nil {
		t.Fatal(err)
	}

	c, err := st.Coverage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.Resources != 2 || c.Vectors != 1 || c.Missing != 1 {
		t.Fatalf("coverage %+v", c)
	}
	if c.FTS != 2 {
		t.Fatalf("fts=%d", c.FTS)
	}

	rows, total, err := st.InspectResources(ctx, "", "", true, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0].ID != "2" || rows[0].HasVec {
		t.Fatalf("missing-only %+v total=%d", rows, total)
	}

	view, err := st.InspectEmbedding(ctx, "boverket:1")
	if err != nil {
		t.Fatal(err)
	}
	if view.Dim != fake.Dim() || view.Norm == 0 {
		t.Fatalf("embed %+v dim want %d", view, fake.Dim())
	}
	_, err = st.InspectEmbedding(ctx, "boverket:2")
	if err != ErrNotFound {
		t.Fatalf("want not found, got %v", err)
	}

	stats, err := st.TableStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 4 {
		t.Fatalf("tables=%d", len(stats))
	}
	meta, err := st.MetaAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta) == 0 {
		t.Fatal("empty meta")
	}
}

func TestInspectRejectsUnknownTable(t *testing.T) {
	st := testStore(t)
	_, err := st.countTable(t.Context(), "sqlite_master")
	if err == nil {
		t.Fatal("expected reject")
	}
}
