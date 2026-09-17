package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mong-x/klimatsearch/internal/config"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

const (
	ToolSearch  = "search_climate_data"
	ToolGet     = "get_resource_details"
	ToolCompare = "compare_materials"
)

// Server wraps the official MCP SDK server and HTTP mounts.
type Server struct {
	MCP       *mcpsdk.Server
	ToolNames []string
	engine    search.SearchEngine
	st        *store.Store
	source    string
	useVector bool
	useRerank bool
}

func New(eng search.SearchEngine, st *store.Store, source string, useVector, useRerank bool) *Server {
	if source == "" {
		source = config.Attribution
	}
	s := &Server{
		MCP: mcpsdk.NewServer(&mcpsdk.Implementation{
			Name:    "klimatsearch",
			Version: "0.1.0",
		}, nil),
		engine:    eng,
		st:        st,
		source:    source,
		useVector: useVector,
		useRerank: useRerank,
	}
	s.register()
	return s
}

func (s *Server) register() {
	mcpsdk.AddTool(s.MCP, &mcpsdk.Tool{
		Name:        ToolSearch,
		Description: "Search Boverket Klimatdatabas generic construction resources by name or description",
	}, s.searchTool)
	mcpsdk.AddTool(s.MCP, &mcpsdk.Tool{
		Name:        ToolGet,
		Description: "Get one Klimatdatabas resource by Resource ID",
	}, s.getTool)
	mcpsdk.AddTool(s.MCP, &mcpsdk.Tool{
		Name:        ToolCompare,
		Description: "Compare A1-A3 typical climate impact and conversions for two resources",
	}, s.compareTool)
	s.ToolNames = []string{ToolSearch, ToolGet, ToolCompare}
}

func (s *Server) Mount(mux *http.ServeMux) {
	stream := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return s.MCP
	}, nil)
	sse := mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server {
		return s.MCP
	}, nil)
	mux.Handle("/mcp", stream)
	mux.Handle("/mcp/sse", sse)
	mux.Handle("/mcp/messages", sse)
}

type searchIn struct {
	Query string `json:"query" jsonschema:"Search query"`
	Lang  string `json:"lang,omitempty" jsonschema:"Language sv or en (default sv)"`
}

type searchOut struct {
	Source  string                `json:"source"`
	Query   string                `json:"query"`
	Lang    string                `json:"lang"`
	Results []search.SearchResult `json:"results"`
}

func (s *Server) searchTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchIn) (*mcpsdk.CallToolResult, searchOut, error) {
	lang, err := search.NormalizeLang(in.Lang)
	if err != nil {
		return nil, searchOut{}, err
	}
	hits, err := s.engine.Search(in.Query, s.useVector, s.useRerank, lang)
	if err != nil {
		return nil, searchOut{}, err
	}
	if hits == nil {
		hits = []search.SearchResult{}
	}
	out := searchOut{Source: s.source, Query: in.Query, Lang: lang, Results: hits}
	return textResult(out), out, nil
}

type getIn struct {
	ID string `json:"id" jsonschema:"Resource ID"`
}

type getOut struct {
	Source   string         `json:"source"`
	Resource map[string]any `json:"resource"`
}

func (s *Server) getTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in getIn) (*mcpsdk.CallToolResult, getOut, error) {
	r, err := s.st.Get(ctx, in.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, getOut{}, fmt.Errorf("resource %s not found", in.ID)
	}
	if err != nil {
		return nil, getOut{}, err
	}
	out := getOut{Source: s.source, Resource: resourceMap(*r, s.source)}
	return textResult(out), out, nil
}

type compareIn struct {
	IDA string `json:"id_a" jsonschema:"First resource ID"`
	IDB string `json:"id_b" jsonschema:"Second resource ID"`
}

type compareOut struct {
	Source        string         `json:"source"`
	A             map[string]any `json:"a"`
	B             map[string]any `json:"b"`
	DeltaA1A3     float64        `json:"delta_a1a3"`
	LowerImpactID string         `json:"lower_impact_id"`
	SameUnit      bool           `json:"same_unit"`
}

func (s *Server) compareTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in compareIn) (*mcpsdk.CallToolResult, compareOut, error) {
	out, err := Compare(ctx, s.st, in.IDA, in.IDB, s.source)
	if err != nil {
		return nil, compareOut{}, err
	}
	return textResult(out), out, nil
}

func Compare(ctx context.Context, st *store.Store, idA, idB, source string) (compareOut, error) {
	a, err := st.Get(ctx, idA)
	if err != nil {
		return compareOut{}, fmt.Errorf("id_a: %w", err)
	}
	b, err := st.Get(ctx, idB)
	if err != nil {
		return compareOut{}, fmt.Errorf("id_b: %w", err)
	}
	lower := a.ResourceID
	if b.A1A3 < a.A1A3 {
		lower = b.ResourceID
	}
	out := compareOut{
		Source:        source,
		A:             resourceMap(*a, source),
		B:             resourceMap(*b, source),
		DeltaA1A3:     a.A1A3 - b.A1A3,
		LowerImpactID: lower,
		SameUnit:      a.Unit == b.Unit,
	}
	return out, nil
}

func resourceMap(r model.Resource, source string) map[string]any {
	conv := r.Conversions
	if conv == nil {
		conv = map[string]float64{}
	}
	return map[string]any{
		"id":             r.ResourceID,
		"name_sv":        r.NameSV,
		"name_en":        r.NameEN,
		"description_sv": r.DescriptionSV,
		"description_en": r.DescriptionEN,
		"a1a3":           r.A1A3,
		"unit":           r.Unit,
		"conversions":    conv,
		"category":       r.Category,
		"version":        r.Version,
		"source":         source,
	}
}

func textResult(v any) *mcpsdk.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		b = []byte("{}")
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(b)}},
	}
}
