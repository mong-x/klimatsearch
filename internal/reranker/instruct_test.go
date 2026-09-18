package reranker

import (
	"strings"
	"testing"
)

func TestIsInstructModel(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"bge-reranker-v2-m3", false},
		{"zerank-1-small", true},
		{"zeroentropy/zerank-1-small", true},
		{"Qwen3-Reranker-0.6B", true},
		{"f2llm-v2-80m", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isInstructModel(tc.name); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestInstructChatTruncates(t *testing.T) {
	q := strings.Repeat("q", 3000)
	p := strings.Repeat("p", 12000)
	prefix, doc, suffix := instructChat(q, p)
	for _, part := range []string{"<|im_start|>system", "<|im_end|>", "<|im_start|>user"} {
		if !strings.Contains(prefix, part) {
			t.Fatalf("prefix missing %q: %q", part, prefix)
		}
	}
	if suffix != "<|im_end|>\n<|im_start|>assistant\n" {
		t.Fatalf("suffix=%q", suffix)
	}
	if len(doc) != 10000 {
		t.Fatalf("passage len %d", len(doc))
	}
	if !strings.Contains(prefix, strings.Repeat("q", 2000)) {
		t.Fatal("query was not truncated to 2000 runes/bytes")
	}
}
