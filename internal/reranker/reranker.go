package reranker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mong-x/klimatsearch/internal/search"
)

// New builds a Reranker. kind is none|fake|onnx.
// onnx loads models/<modelName>/model.int8.onnx (KLIMAT_ONNX_QUANT=auto) or model.onnx.
// Names containing zerank or qwen use instruct encode; others use BGE pair encode.
func New(kind, modelsDir, modelName, quant string) (search.Reranker, error) {
	switch strings.ToLower(kind) {
	case "", "none":
		return None{}, nil
	case "fake":
		return Fake{}, nil
	case "onnx":
		return newONNX(modelsDir, modelName, quant)
	default:
		return nil, fmt.Errorf("unknown reranker %q", kind)
	}
}

// None leaves ranking unchanged.
type None struct{}

func (None) Rerank(_ string, docs []search.Hit) ([]search.Hit, error) {
	return docs, nil
}

// Fake reorders by case-insensitive overlap of query tokens with names.
type Fake struct{}

func (Fake) Rerank(query string, docs []search.Hit) ([]search.Hit, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(q)
	out := append([]search.Hit(nil), docs...)
	type scored struct {
		idx   int
		boost float64
	}
	boosts := make([]scored, len(out))
	for i, d := range out {
		hay := strings.ToLower(d.EmbeddingText())
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
