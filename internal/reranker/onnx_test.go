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

func TestONNXRerankIfModelPresent(t *testing.T) {
	root := filepath.Join("..", "..")
	dir := filepath.Join(root, "models", "bge-reranker-v2-m3")
	if _, err := os.Stat(filepath.Join(dir, "model.onnx")); err != nil {
		t.Skip("bge-reranker-v2-m3 ONNX not present")
	}
	if _, err := os.Stat(filepath.Join(dir, "tokenizer.json")); err != nil {
		t.Skip("tokenizer.json not present")
	}
	rr, err := New("onnx", filepath.Join(root, "models"), "bge-reranker-v2-m3", "auto")
	if err != nil {
		t.Fatal(err)
	}
	docs := []search.Hit{
		HitFromRes(model.Resource{ResourceID: "noise", NameSV: "Kopparrör", NameEN: "Copper pipe"}),
		HitFromRes(model.Resource{ResourceID: "hit", NameSV: "Spånskiva", NameEN: "Particle board", DescriptionSV: "Träbaserad skiva"}),
	}
	out, err := rr.Rerank("spånskiva", docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ResourceID != "hit" {
		t.Fatalf("expected particle board first, got %+v", out)
	}
}

func HitFromRes(r model.Resource) search.Hit {
	return search.HitFrom(r, "sv", "fts", 0)
}
