package embedder

import "fmt"

// Tokenizer turns text into ONNX input ids. Production uses a HuggingFace
// tokenizer.json next to model.onnx; the stub fails with a clear error.
type Tokenizer interface {
	Encode(text string) (inputIDs []int64, attentionMask []int64, err error)
}

// StubTokenizer is shipped so ONNX code compiles without tokenizers CGO.
type StubTokenizer struct {
	Path string
}

func (t StubTokenizer) Encode(string) ([]int64, []int64, error) {
	path := t.Path
	if path == "" {
		path = "tokenizer.json"
	}
	return nil, nil, fmt.Errorf("onnx tokenizer not configured (missing %s); place HuggingFace tokenizer.json next to model.onnx — see scripts/download-models.sh", path)
}

// FileTokenizer is a placeholder that still requires a real tokenizer impl.
// v1 fails closed: presence of tokenizer.json is not enough without a wired
// HuggingFace runtime, so Encode explains how to finish the production path.
type FileTokenizer struct {
	Path string
}

func (t FileTokenizer) Encode(string) ([]int64, []int64, error) {
	return nil, nil, fmt.Errorf("onnx tokenizer file found at %s but HuggingFace tokenization is not wired in this build; use --embedder=fake or add a Tokenizer implementation", t.Path)
}
