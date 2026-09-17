package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/hash"
	"github.com/mong-x/klimatsearch/internal/mcp"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

func TestToolsRegistered(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "mcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	a := model.Resource{ResourceID: "a", NameSV: "Betong", NameEN: "Concrete", A1A3: 0.12, Unit: "kg", Conversions: map[string]float64{"kg/m³": 2400}}
	b := model.Resource{ResourceID: "b", NameSV: "Stål", NameEN: "Steel", A1A3: 1.5, Unit: "kg"}
	for _, r := range []model.Resource{a, b} {
		h, _ := hash.Content(r)
		r.ContentHash = h
		vec, _ := fake.Embed(r.EmbeddingText())
		if err := st.Upsert(t.Context(), r, vec); err != nil {
			t.Fatal(err)
		}
	}
	eng := search.New(st, st, fake, reranker.None{})
	s := mcp.New(eng, st, "Boverket Klimatdatabas", true, false)
	want := map[string]bool{mcp.ToolSearch: false, mcp.ToolGet: false, mcp.ToolCompare: false}
	for _, n := range s.ToolNames {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, ok := range want {
		if !ok {
			t.Fatalf("missing tool %s in ToolNames=%v", n, s.ToolNames)
		}
	}

	ctx := t.Context()
	t1, t2 := mcpsdk.NewInMemoryTransports()
	if _, err := s.MCP.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range listed.Tools {
		got[tool.Name] = true
	}
	for n := range want {
		if !got[n] {
			t.Fatalf("server ListTools missing %s: %#v", n, got)
		}
	}

	cmp, err := mcp.Compare(ctx, st, "a", "b", "Boverket Klimatdatabas")
	if err != nil {
		t.Fatal(err)
	}
	if cmp.LowerImpactID != "a" {
		t.Fatalf("lower=%s", cmp.LowerImpactID)
	}
	if cmp.DeltaA1A3 != 0.12-1.5 {
		t.Fatalf("delta=%v", cmp.DeltaA1A3)
	}
}
