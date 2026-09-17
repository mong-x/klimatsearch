package reranker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mong-x/klimatsearch/internal/search"
)

// New builds a Reranker. kind is none|fake|onnx.
func New(kind, modelsDir, modelName string) (search.Reranker, error) {
	switch strings.ToLower(kind) {
	case "", "none":
		return None{}, nil
	case "fake":
		return Fake{}, nil
	case "onnx":
		p := filepath.Join(modelsDir, modelName, "reranker.onnx")
		return NewONNX(p), nil
	default:
		return nil, fmt.Errorf("unknown reranker %q", kind)
	}
}

// None leaves ranking unchanged.
type None struct{}

func (None) Rerank(_ string, docs []search.SearchResult) ([]search.SearchResult, error) {
	return docs, nil
}

// Fake reorders by case-insensitive overlap of query tokens with names.
type Fake struct{}

func (Fake) Rerank(query string, docs []search.SearchResult) ([]search.SearchResult, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(q)
	out := append([]search.SearchResult(nil), docs...)
	type scored struct {
		idx   int
		boost float64
	}
	boosts := make([]scored, len(out))
	for i, d := range out {
		hay := strings.ToLower(d.NameSV + " " + d.NameEN + " " + d.DescriptionSV + " " + d.DescriptionEN)
		var b float64
		if q != "" && strings.Contains(hay, q) {
			b += 10
		}
		for _, tok := range tokens {
			if tok != "" && strings.Contains(hay, tok) {
				b++
			}
		}
		boosts[i] = scored{idx: i, boost: b}
		out[i].Score = d.Score + b
		out[i].Source = "rerank"
	}
	sort.SliceStable(out, func(i, j int) bool {
		if boosts[i].boost != boosts[j].boost {
			return boosts[i].boost > boosts[j].boost
		}
		return out[i].Score > out[j].Score
	})
	return out, nil
}

// ONNX is a compile-ready stub that fails until a reranker.onnx is present
// and wired. Tests must not select kind=onnx.
type ONNX struct {
	mu        sync.Mutex
	modelPath string
	inited    bool
	initErr   error
}

func NewONNX(modelPath string) *ONNX {
	return &ONNX{modelPath: modelPath}
}

func (o *ONNX) Rerank(query string, docs []search.SearchResult) ([]search.SearchResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.inited {
		o.inited = true
		if _, err := os.Stat(o.modelPath); err != nil {
			o.initErr = fmt.Errorf("onnx reranker %s: %w", o.modelPath, err)
		} else {
			o.initErr = fmt.Errorf("onnx reranker found at %s but inference is not wired in this build; use --reranker=none or fake", o.modelPath)
		}
	}
	if o.initErr != nil {
		return nil, o.initErr
	}
	_ = query
	return docs, nil
}
