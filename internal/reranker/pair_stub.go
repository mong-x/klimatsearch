//go:build !tokenizers

package reranker

import "fmt"

func encodePairHF(path, query, passage string, maxLen int) ([]int64, []int64, error) {
	_ = query
	_ = passage
	_ = maxLen
	if path == "" {
		path = "tokenizer.json"
	}
	return nil, nil, fmt.Errorf("rebuild with -tags tokenizers and libtokenizers.a; tokenizer file=%s", path)
}
