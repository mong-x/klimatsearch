//go:build tokenizers

package reranker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeInstructChatTemplate(t *testing.T) {
	// F2LLM tokenizer is Qwen-based (same im_start/im_end ids as zerank).
	path := filepath.Join("..", "..", "models", "f2llm-v2-80m", "tokenizer.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("tokenizer.json not present")
	}
	ids, mask, err := encodeInstruct(path, "spånskiva", "Particle board used in construction")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 || len(ids) != len(mask) {
		t.Fatalf("ids=%d mask=%d", len(ids), len(mask))
	}
	const imStart, imEnd = 151644, 151645
	var sawStart, sawEnd bool
	for _, id := range ids {
		if id == imStart {
			sawStart = true
		}
		if id == imEnd {
			sawEnd = true
		}
	}
	if !sawStart || !sawEnd {
		t.Fatalf("expected im_start/im_end in ids=%v", ids)
	}
	if ids[len(ids)-1] == imEnd {
		t.Fatalf("last token should be assistant header, not im_end: %v", ids)
	}
	if len(ids) > maxInstructLen {
		t.Fatalf("len %d > %d", len(ids), maxInstructLen)
	}
}
