package search

import (
	"net/url"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Defaults are process Vector/Rerank when the caller omits them.
// REST and HTML use false,false. MCP uses the process Embedder/Reranker config.
type Defaults struct {
	Vector bool
	Rerank bool
}

// FromURL builds a Query from HTTP query values (REST and the operator console).
func FromURL(qs url.Values, def Defaults) (Query, error) {
	lang, err := NormalizeLang(qs.Get("lang"))
	if err != nil {
		return Query{}, err
	}
	_, hasV := qs["vector"]
	_, hasR := qs["rerank"]
	return Query{
		Text:     strings.TrimSpace(qs.Get("q")),
		Lang:     lang,
		Catalogs: model.ParseCatalogIDs(qs["databases"]),
		Vector:   Coalesce(ParseFlag(qs.Get("vector"), hasV), def.Vector),
		Rerank:   Coalesce(ParseFlag(qs.Get("rerank"), hasR), def.Rerank),
	}, nil
}

// FromMCP builds a Query from MCP tool fields.
func FromMCP(text, lang string, databases []string, vector, rerank *bool, def Defaults) (Query, error) {
	l, err := NormalizeLang(lang)
	if err != nil {
		return Query{}, err
	}
	return Query{
		Text:     strings.TrimSpace(text),
		Lang:     l,
		Catalogs: model.ParseCatalogIDs(databases),
		Vector:   Coalesce(vector, def.Vector),
		Rerank:   Coalesce(rerank, def.Rerank),
	}, nil
}
