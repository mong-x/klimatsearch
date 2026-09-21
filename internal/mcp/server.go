package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mong-x/klimatsearch/internal/compare"
	"github.com/mong-x/klimatsearch/internal/config"
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

// ReadOnlyAnnotations describes the klimatsearch tools: every tool reads the
// local Klimatdatabas store and never mutates it or reaches the network, so
// the four hints must stay read-only, non-destructive, idempotent, closed
// world. OpenAI's tool directory rejects tools where any hint is missing or
// non-boolean, so all four are set explicitly.
func ReadOnlyAnnotations() *mcpsdk.ToolAnnotations {
	no := false
	return &mcpsdk.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: &no,
		IdempotentHint:  true,
		OpenWorldHint:   &no,
	}
}

func (s *Server) register() {
	mcpsdk.AddTool(s.MCP, &mcpsdk.Tool{
		Name:        ToolSearch,
		Description: "Search Klimatdatabas. Hits are full Resources (typical A1A3, conservative A1A3, A4, A5.1, transports when published). Optional vector/rerank override the process defaults (omit to use server config).",
		Annotations: ReadOnlyAnnotations(),
	}, s.searchTool)
	mcpsdk.AddTool(s.MCP, &mcpsdk.Tool{
		Name:        ToolGet,
		Description: "Get one Klimatdatabas Resource by Resource ID or catalog:id. Includes origin (Boverket product sheet URL) when known. HTTP GET {details}/origin 302s there.",
		Annotations: ReadOnlyAnnotations(),
	}, s.getTool)
	mcpsdk.AddTool(s.MCP, &mcpsdk.Tool{
		Name:        ToolCompare,
		Description: "Compare climate impact of two Resources in a shared unit. impact=typical (default)|conservative|a4|a5_1",
		Annotations: ReadOnlyAnnotations(),
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
	Vector    *bool    `json:"vector,omitempty" jsonschema:"Override process default; false is FTS only"`
	Rerank    *bool    `json:"rerank,omitempty" jsonschema:"Override process default; BGE INT8 is slow"`
}

type searchOut struct {
	Source  string           `json:"source"`
	Query   string           `json:"query"`
	Lang    string           `json:"lang"`
	Results []map[string]any `json:"results"`
}

func (s *Server) searchTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchIn) (*mcpsdk.CallToolResult, searchOut, error) {
	q, err := search.FromMCP(in.Query, in.Lang, in.Databases, in.Vector, in.Rerank, search.Defaults{Vector: s.useVector, Rerank: s.useRerank})
	if err != nil {
		return nil, searchOut{}, err
	}
	hits, err := s.engine.Search(ctx, q)
	if err != nil {
		return nil, searchOut{}, err
	}
	env := search.Envelope(s.source, q, hits)
	out := searchOut{
		Source:  s.source,
		Query:   in.Query,
		Lang:    q.Lang,
		Results: env["results"].([]map[string]any),
	}
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
	IDA    string `json:"id_a" jsonschema:"First resource ID"`
	IDB    string `json:"id_b" jsonschema:"Second resource ID"`
	Unit   string `json:"unit,omitempty" jsonschema:"Optional unit to compare in"`
	Impact string `json:"impact,omitempty" jsonschema:"typical (default), conservative, a4, or a5_1"`
}

func (s *Server) compareTool(ctx context.Context, _ *mcpsdk.CallToolRequest, in compareIn) (*mcpsdk.CallToolResult, map[string]any, error) {
	cmp, err := compare.Resources(ctx, s.st, in.IDA, in.IDB, s.source, in.Unit, in.Impact)
	if err != nil {
		var side *compare.SideError
		if errors.As(err, &side) {
			return nil, nil, fmt.Errorf("id_%s: %w", side.Side, err)
		}
		return nil, nil, err
	}
	view := cmp.View()
	return textResult(view), view, nil
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
