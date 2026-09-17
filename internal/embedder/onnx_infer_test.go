//go:build tokenizers

package embedder

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestONNXEmbedIfModelPresent(t *testing.T) {
	dir := filepath.Join("..", "..", "models", "f2llm-v2-80m")
	onnx := filepath.Join(dir, "model.onnx")
	tok := filepath.Join(dir, "tokenizer.json")
	if _, err := os.Stat(onnx); err != nil {
		t.Skip("model.onnx not present")
	}
	if _, err := os.Stat(tok); err != nil {
		t.Skip("tokenizer.json not present")
	}
	e, err := NewONNX(onnx, FileTokenizer{Path: tok}, Dim)
	if err != nil {
		t.Fatal(err)
	}
	vec, err := e.Embed("name_sv: Spånskiva")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != Dim {
		t.Fatalf("dim %d", len(vec))
	}
	var sum float64
	for _, x := range vec {
		sum += float64(x) * float64(x)
	}
	if math.Abs(sum-1) > 1e-3 {
		t.Fatalf("not L2-normalized: %v", sum)
	}
	q, err := e.EmbedQuery("spånskiva")
	if err != nil {
		t.Fatal(err)
	}
	if len(q) != Dim {
		t.Fatalf("query dim %d", len(q))
	}
}
