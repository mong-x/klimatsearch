package search

// SearchResult is one hit from the SearchEngine.
type SearchResult struct {
	ID            string
	NameSV        string
	NameEN        string
	DescriptionSV string
	DescriptionEN string
	A1A3          float64
	Unit          string
	Lang          string
	Score         float64
	Source        string // "fts" | "vector" | "rerank"
}

// Embedder turns text into a fixed-dimension vector.
type Embedder interface {
	Embed(text string) ([]float32, error)
}

// Reranker reorders SearchEngine results after retrieval.
type Reranker interface {
	Rerank(query string, docs []SearchResult) ([]SearchResult, error)
}

// SearchEngine is hybrid retrieval over Klimatdatabas.
type SearchEngine interface {
	Search(query string, useVector bool, useRerank bool, lang string) ([]SearchResult, error)
}

// Dimensional is implemented by embedders that know their output width.
type Dimensional interface {
	Dim() int
}
