package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mong-x/klimatsearch/internal/mcp"
	"github.com/mong-x/klimatsearch/internal/testworld"
)

// newTestServer seeds the canonical pair (testworld) and returns a client
// session connected to the MCP server over an in-memory transport.
func newTestServer(t *testing.T) *mcpsdk.ClientSession {
	t.Helper()
	st, eng, _ := testworld.Seed(t)
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
	if out.Results[0]["id"] != testworld.Betong.ResourceID {
		t.Fatalf("top hit id=%v, want %s", out.Results[0]["id"], testworld.Betong.ResourceID)
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
	callTool(t, cs, "get_resource_details", map[string]any{"id": testworld.Betong.ResourceID}, &out)
	if out.Resource["id"] != testworld.Betong.ResourceID {
		t.Fatalf("resource id=%v, want %s", out.Resource["id"], testworld.Betong.ResourceID)
	}
	if out.Resource["name_sv"] != "Betong" {
		t.Fatalf("name_sv=%v", out.Resource["name_sv"])
	}
}

func TestCompareResourcesTool(t *testing.T) {
	cs := newTestServer(t)
	var out map[string]any
	callTool(t, cs, "compare_resources", map[string]any{"id_a": testworld.Betong.ResourceID, "id_b": testworld.Stal.ResourceID}, &out)
	if out["lower_impact_id"] != testworld.Betong.ResourceID {
		t.Fatalf("lower_impact_id=%v, want %s", out["lower_impact_id"], testworld.Betong.ResourceID)
	}
}

// TestCompareResourcesToolErrorPrefix pins the MCP-side rendering of a
// missing side: the tool result errors with the id_a:/id_b: prefix built
// from compare.SideError.
func TestCompareResourcesToolErrorPrefix(t *testing.T) {
	cs := newTestServer(t)
	res, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
		Name:      "compare_resources",
		Arguments: map[string]any{"id_a": testworld.Betong.ResourceID, "id_b": "missing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error, got %+v", res.Content)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			text = tc.Text
		}
	}
	if !strings.Contains(text, "id_b:") {
		t.Fatalf("error text %q must name the failing side", text)
	}
}
