//go:build tokenizers

package reranker

import (
	"fmt"
	"os"

	"github.com/daulet/tokenizers"
)

// encodeInstructHF tokenizes zerank/Qwen chat: system=query, user=passage,
// add_generation_prompt. Last token is the assistant header so the graph can
// emit the Yes logit. add_special_tokens is off — the template already has them.
func encodeInstructHF(path, query, passage string, maxLen int) ([]int64, []int64, error) {
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

	prefix, doc, suffix := instructChat(query, passage)
	pre := tk.EncodeWithOptions(prefix, false, tokenizers.WithReturnAttentionMask())
	suf := tk.EncodeWithOptions(suffix, false, tokenizers.WithReturnAttentionMask())
	pass := tk.EncodeWithOptions(doc, false, tokenizers.WithReturnAttentionMask())
	if len(pre.IDs) == 0 || len(suf.IDs) == 0 {
		return nil, nil, fmt.Errorf("tokenizer produced no instruct tokens")
	}
	keepPass := len(pass.IDs)
	if maxLen > 0 {
		budget := maxLen - len(pre.IDs) - len(suf.IDs)
		if budget < 0 {
			budget = 0
		}
		if keepPass > budget {
			keepPass = budget
		}
	}
	ids32 := make([]uint32, 0, len(pre.IDs)+keepPass+len(suf.IDs))
	ids32 = append(ids32, pre.IDs...)
	ids32 = append(ids32, pass.IDs[:keepPass]...)
	ids32 = append(ids32, suf.IDs...)
	ids := make([]int64, len(ids32))
	mask := make([]int64, len(ids32))
	for i, id := range ids32 {
		ids[i] = int64(id)
		mask[i] = 1
	}
	return ids, mask, nil
}
