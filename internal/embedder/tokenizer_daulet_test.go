//go:build tokenizers

package embedder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeHFAddsEOS(t *testing.T) {
	path := filepath.Join("..", "..", "models", "f2llm-v2-80m", "tokenizer.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("tokenizer.json not present (gitignored weights)")
	}
	ids, mask, err := encodeHF(path, "name_sv: Spånskiva")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 || len(ids) != len(mask) {
		t.Fatalf("ids=%d mask=%d", len(ids), len(mask))
	}
	const eos = 151645 // F2LLM-v2-80M <|im_end|>
	if ids[len(ids)-1] != eos {
		t.Fatalf("last token %d want eos %d ids=%v", ids[len(ids)-1], eos, ids)
	}
	if lastTokenIndex(mask) != len(mask)-1 {
		t.Fatalf("eos index %d", lastTokenIndex(mask))
	}
}
