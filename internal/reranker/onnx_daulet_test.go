//go:build tokenizers

package reranker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/search"
)

// TestONNXRerankIfModelPresent needs the real HF tokenizer, which requires
// -tags tokenizers and libtokenizers.a. Without the tag encodeInstructHF is
// a stub that always errors, so the test lives behind the build tag like the
// other tokenizer-dependent tests.
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
