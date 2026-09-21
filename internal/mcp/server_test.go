package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/mcp"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

// newTestServer opens a store with two resources, starts the MCP server on an
// in-memory transport, and returns a connected client session.
func newTestServer(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
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
	b := model.Resource{ResourceID: "b", NameSV: "Stål", NameEN: "Steel", A1A3: 1.5, Unit: "kg", Conversions: map[string]float64{"kg/m³": 7800}}
	for _, r := range []model.Resource{a, b} {
		h, _ := r.ContentHash()
		r.Hash = h
		vec, _ := fake.Embed(r.EmbeddingText())
		if err := st.Upsert(t.Context(), r, vec); err != nil {
			t.Fatal(err)
		}
	}
	eng := search.New(st, st, fake, reranker.None{})
	s := mcp.New(eng, st, "Boverket Klimatdatabas", true, false)

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
	return cs
}

// callTool invokes one tool by name and decodes its text content into out.
func callTool(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s returned tool error: %+v", name, res.Content)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			text = tc.Text
		}
	}
	if text == "" {
		t.Fatalf("%s returned no text content", name)
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("%s: decode %q: %v", name, text, err)
	}
}

func TestToolsRegistered(t *testing.T) {
	cs := newTestServer(t)
	listed, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{mcp.ToolSearch: false, mcp.ToolGet: false, mcp.ToolCompare: false}
	got := map[string]bool{}
	for _, tool := range listed.Tools {
		got[tool.Name] = true
	}
	for n := range want {
		if !got[n] {
			t.Fatalf("server ListTools missing %s: %#v", n, got)
		}
	}
}

// TestToolAnnotationsHints pins the four annotation hints on every tool.
// Host directories (e.g. OpenAI's) reject tools where any of readOnlyHint,
// destructiveHint, idempotentHint, or openWorldHint is missing or non-boolean,
// so each must serialize as an explicit JSON boolean. All klimatsearch tools
// read the local store only: read-only, non-destructive, idempotent, closed
// world.
func TestToolAnnotationsHints(t *testing.T) {
	cs := newTestServer(t)
	listed, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) == 0 {
		t.Fatal("no tools listed")
	}
	for _, tool := range listed.Tools {
		if tool.Annotations == nil {
			t.Fatalf("%s: missing annotations", tool.Name)
		}
		b, err := json.Marshal(tool.Annotations)
		if err != nil {
			t.Fatalf("%s: marshal annotations: %v", tool.Name, err)
		}
		var hints map[string]any
		if err := json.Unmarshal(b, &hints); err != nil {
			t.Fatalf("%s: decode annotations %s: %v", tool.Name, b, err)
		}
		for _, name := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			v, ok := hints[name]
			if !ok {
				t.Errorf("%s: annotations missing %s in %s", tool.Name, name, b)
				continue
			}
			if _, isBool := v.(bool); !isBool {
				t.Errorf("%s: annotation %s is %T, want boolean", tool.Name, name, v)
			}
		}
		if hints["readOnlyHint"] != true {
			t.Errorf("%s: readOnlyHint=%v, want true", tool.Name, hints["readOnlyHint"])
		}
		if hints["destructiveHint"] != false {
			t.Errorf("%s: destructiveHint=%v, want false", tool.Name, hints["destructiveHint"])
		}
		if hints["idempotentHint"] != true {
			t.Errorf("%s: idempotentHint=%v, want true", tool.Name, hints["idempotentHint"])
		}
		if hints["openWorldHint"] != false {
			t.Errorf("%s: openWorldHint=%v, want false", tool.Name, hints["openWorldHint"])
		}
	}
}

func TestSearchClimateDataTool(t *testing.T) {
	cs := newTestServer(t)
	var out struct {
		Source  string           `json:"source"`
		Query   string           `json:"query"`
		Lang    string           `json:"lang"`
		Results []map[string]any `json:"results"`
	}
	callTool(t, cs, "search_climate_data", map[string]any{"query": "betong"}, &out)
	if len(out.Results) == 0 {
		t.Fatal("search_climate_data returned no results for betong")
	}
	if out.Results[0]["id"] != "a" {
		t.Fatalf("top hit id=%v, want a", out.Results[0]["id"])
	}
	if out.Source != "Boverket Klimatdatabas" {
		t.Fatalf("source=%s", out.Source)
	}
}

func TestGetResourceDetailsTool(t *testing.T) {
	cs := newTestServer(t)
	var out struct {
		Source   string         `json:"source"`
		Resource map[string]any `json:"resource"`
	}
	callTool(t, cs, "get_resource_details", map[string]any{"id": "a"}, &out)
	if out.Resource["id"] != "a" {
		t.Fatalf("resource id=%v, want a", out.Resource["id"])
	}
	if out.Resource["name_sv"] != "Betong" {
		t.Fatalf("name_sv=%v", out.Resource["name_sv"])
	}
}

func TestCompareResourcesTool(t *testing.T) {
	cs := newTestServer(t)
	var out map[string]any
	callTool(t, cs, "compare_resources", map[string]any{"id_a": "a", "id_b": "b"}, &out)
	if out["lower_impact_id"] != "a" {
		t.Fatalf("lower_impact_id=%v, want a", out["lower_impact_id"])
	}
}

func TestCompare(t *testing.T) {
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
	b := model.Resource{ResourceID: "b", NameSV: "Stål", NameEN: "Steel", A1A3: 1.5, Unit: "kg", Conversions: map[string]float64{"kg/m³": 7800}}
	for _, r := range []model.Resource{a, b} {
		h, _ := r.ContentHash()
		r.Hash = h
		vec, _ := fake.Embed(r.EmbeddingText())
		if err := st.Upsert(t.Context(), r, vec); err != nil {
			t.Fatal(err)
		}
	}

	ctx := t.Context()
	cmp, err := mcp.Compare(ctx, st, "a", "b", "Boverket Klimatdatabas", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cmp.LowerImpactID != "a" {
		t.Fatalf("lower=%s", cmp.LowerImpactID)
	}
	if cmp.DeltaA1A3 != 0.12-1.5 {
		t.Fatalf("delta=%v", cmp.DeltaA1A3)
	}
	if cmp.Incomparable {
		t.Fatal("same kg unit should be comparable")
	}
	if cmp.Unit != "kg" {
		t.Fatalf("unit=%s", cmp.Unit)
	}

	_, err = mcp.Compare(ctx, st, "a", "b", "Boverket Klimatdatabas", "m²", "")
	if err == nil {
		t.Fatal("explicit missing unit should error")
	}

	cmp, err = mcp.Compare(ctx, st, "a", "b", "Boverket Klimatdatabas", "m³", "")
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Unit != "m³" {
		t.Fatalf("unit=%s", cmp.Unit)
	}
}
