//go:build !tokenizers

package embedder

import "fmt"

func encodeHF(path, text string) ([]int64, []int64, error) {
	_ = text
	if path == "" {
		path = "tokenizer.json"
	}
	return nil, nil, fmt.Errorf("rebuild with -tags tokenizers and libtokenizers.a (scripts/fetch-libtokenizers.sh); tokenizer file=%s — or use --embedder=fake", path)
}
