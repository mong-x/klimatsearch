package embedder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mong-x/klimatsearch/internal/search"
)

var (
	_ search.Embedder = Fake{}
	_ search.Embedder = (*ONNX)(nil)
)

// New builds an Embedder. kind is auto|fake|onnx.
// auto uses onnx when model.int8.onnx or model.onnx exists, otherwise fake.
// quant is auto|int8|fp32 (empty = auto).
func New(kind, modelsDir, modelName, quant string) (search.Embedder, error) {
	dir := filepath.Join(modelsDir, modelName)
	onnxPath, resErr := ResolveONNX(dir, quant)
	switch kind {
	case "", "auto":
		if resErr == nil {
			kind = "onnx"
		} else {
			kind = "fake"
		}
	}
	switch kind {
	case "fake":
		return Fake{}, nil
	case "onnx":
		if resErr != nil {
			return nil, fmt.Errorf("embedder %w (see docs/SELFHOST.md)", resErr)
		}
		tokPath := filepath.Join(dir, "tokenizer.json")
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
