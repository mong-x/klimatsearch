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
		if !fileExists(onnxPath) {
			return nil, fmt.Errorf("embedder onnx: missing %s (see docs/SELFHOST.md)", onnxPath)
		}
		tokPath := filepath.Join(modelsDir, modelName, "tokenizer.json")
		if !fileExists(tokPath) {
			return nil, fmt.Errorf("embedder onnx: missing %s (Hugging Face tokenizer.json is required)", tokPath)
		}
		return NewONNX(onnxPath, FileTokenizer{Path: tokPath}, Dim)
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
