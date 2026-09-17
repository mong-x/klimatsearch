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
	ToolCompare = "compare_resources"
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
		Description: "Compare A1-A3 typical climate impact of two resources in a shared unit",
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
	Query     string   `json:"query" jsonschema:"Search query"`
	Lang      string   `json:"lang,omitempty" jsonschema:"Language sv or en (default sv)"`
	Databases []string `json:"databases,omitempty" jsonschema:"Optional Catalog IDs to search"`
}

type searchOut struct {
	Source  string       `json:"source"`
	Query   string       `json:"query"`
	Lang    string       `json:"lang"`
	Results []search.Hit `json:"results"`
}

func (s *Server) searchTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchIn) (*mcpsdk.CallToolResult, searchOut, error) {
	lang, err := search.NormalizeLang(in.Lang)
	if err != nil {
		return nil, searchOut{}, err
	}
	hits, err := s.engine.Search(ctx, in.Query, s.useVector, s.useRerank, lang, in.Databases)
	if err != nil {
		return nil, searchOut{}, err
	}
	if hits == nil {
		hits = []search.Hit{}
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
	if errors.Is(err, store.ErrAmbiguous) {
		return nil, getOut{}, err
	}
	if err != nil {
		return nil, getOut{}, err
	}
	out := getOut{Source: s.source, Resource: r.View(s.source)}
	return textResult(out), out, nil
}

type compareIn struct {
	IDA  string `json:"id_a" jsonschema:"First resource ID"`
	IDB  string `json:"id_b" jsonschema:"Second resource ID"`
	Unit string `json:"unit,omitempty" jsonschema:"Optional unit to compare in"`
}

type compareOut struct {
	Source        string         `json:"source"`
	A             map[string]any `json:"a"`
	B             map[string]any `json:"b"`
	Unit          string         `json:"unit"`
	DeltaA1A3     float64        `json:"delta_a1a3"`
	LowerImpactID string         `json:"lower_impact_id"`
	Incomparable  bool           `json:"incomparable"`
}

func (s *Server) compareTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in compareIn) (*mcpsdk.CallToolResult, compareOut, error) {
	out, err := Compare(ctx, s.st, in.IDA, in.IDB, s.source, in.Unit)
	if err != nil {
		return nil, compareOut{}, err
	}
	return textResult(out), out, nil
}

func Compare(ctx context.Context, st *store.Store, idA, idB, source, unit string) (compareOut, error) {
	a, err := st.Get(ctx, idA)
	if err != nil {
		return compareOut{}, fmt.Errorf("id_a: %w", err)
	}
	b, err := st.Get(ctx, idB)
	if err != nil {
		return compareOut{}, fmt.Errorf("id_b: %w", err)
	}
	cmp, err := model.Compare(*a, *b, source, unit)
	if err != nil {
		return compareOut{}, err
	}
	return compareOut{
		Source:        cmp.Attribution,
		A:             cmp.A,
		B:             cmp.B,
		Unit:          cmp.Unit,
		DeltaA1A3:     cmp.DeltaA1A3,
		LowerImpactID: cmp.LowerImpactID,
		Incomparable:  cmp.Incomparable,
	}, nil
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
