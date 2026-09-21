package reranker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
)

func TestNewONNXMissingModel(t *testing.T) {
	_, err := New("onnx", t.TempDir(), "missing", "auto")
	if err == nil {
		t.Fatal("expected error when reranker model.onnx is absent")
	}
}

func TestNewONNXInt8Missing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "m")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.onnx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tokenizer.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := New("onnx", root, "m", "int8")
	if err == nil {
		t.Fatal("expected error when model.int8.onnx is required but absent")
	}
}

func HitFromRes(r model.Resource) search.Hit {
	return search.HitFrom(r, "sv", "fts", 0)
}
