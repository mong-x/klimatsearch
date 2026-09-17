//go:build tokenizers

package embedder

import (
	"fmt"
	"os"

	"github.com/daulet/tokenizers"
)

func encodeHF(path, text string) ([]int64, []int64, error) {
	if path == "" {
		return nil, nil, fmt.Errorf("tokenizer.json path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, nil, fmt.Errorf("tokenizer %s: %w", path, err)
	}
	tk, err := tokenizers.FromFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("load tokenizer %s: %w", path, err)
	}
	defer tk.Close()
	enc := tk.EncodeWithOptions(text, true, tokenizers.WithReturnAttentionMask())
	if len(enc.IDs) == 0 {
		return nil, nil, fmt.Errorf("tokenizer produced no tokens")
	}
	ids := make([]int64, len(enc.IDs))
	mask := make([]int64, len(enc.IDs))
	for i, id := range enc.IDs {
		ids[i] = int64(id)
		if i < len(enc.AttentionMask) {
			mask[i] = int64(enc.AttentionMask[i])
		} else {
			mask[i] = 1
		}
	}
	return ids, mask, nil
}
