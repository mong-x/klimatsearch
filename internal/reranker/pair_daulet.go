//go:build tokenizers

package reranker

import (
	"fmt"
	"os"

	"github.com/daulet/tokenizers"
)

// encodePairHF builds XLM-RoBERTa pair ids: <s> query </s></s> passage </s>
func encodePairHF(path, query, passage string, maxLen int) ([]int64, []int64, error) {
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

	qEnc := tk.EncodeWithOptions(query, true, tokenizers.WithReturnAttentionMask())
	if len(qEnc.IDs) == 0 {
		return nil, nil, fmt.Errorf("tokenizer produced no query tokens")
	}
	pEnc := tk.EncodeWithOptions(passage, false, tokenizers.WithReturnAttentionMask())
	sep := qEnc.IDs[len(qEnc.IDs)-1]
	ids32 := make([]uint32, 0, len(qEnc.IDs)+2+len(pEnc.IDs))
	ids32 = append(ids32, qEnc.IDs...)
	ids32 = append(ids32, sep)
	ids32 = append(ids32, pEnc.IDs...)
	ids32 = append(ids32, sep)
	if maxLen > 0 && len(ids32) > maxLen {
		// Keep <s> query </s></s> prefix; truncate the passage tail including final </s>, then re-append </s>.
		keep := maxLen - 1
		if keep < len(qEnc.IDs)+1 {
			keep = maxLen
			ids32 = ids32[:keep]
		} else {
			ids32 = append(ids32[:keep], sep)
		}
	}
	ids := make([]int64, len(ids32))
	mask := make([]int64, len(ids32))
	for i, id := range ids32 {
		ids[i] = int64(id)
		mask[i] = 1
	}
	return ids, mask, nil
}
