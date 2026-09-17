package embedder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mong-x/klimatsearch/internal/search"
)

var (
	_ search.Embedder      = Fake{}
	_ search.QueryEmbedder = (*ONNX)(nil)
)

// New builds an Embedder. kind is auto|fake|onnx.
// auto uses onnx when model.onnx exists, otherwise fake.
func New(kind, modelsDir, modelName string) (search.Embedder, error) {
	onnxPath := filepath.Join(modelsDir, modelName, "model.onnx")
	switch kind {
	case "", "auto":
		if fileExists(onnxPath) {
			kind = "onnx"
		} else {
			kind = "fake"
		}
	}
	switch kind {
	case "fake":
		return Fake{}, nil
	case "onnx":
		tokPath := filepath.Join(modelsDir, modelName, "tokenizer.json")
		var tok Tokenizer = StubTokenizer{Path: tokPath}
		if fileExists(tokPath) {
			tok = FileTokenizer{Path: tokPath}
		}
		return NewONNX(onnxPath, tok, Dim)
	default:
		return nil, fmt.Errorf("unknown embedder %q", kind)
	}
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func DimOf(e search.Embedder) int {
	if d, ok := e.(search.Dimensional); ok {
		return d.Dim()
	}
	return Dim
}
