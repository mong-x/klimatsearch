package reranker

import "strings"

const maxInstructLen = 512

func encodeInstruct(path, query, passage string) (ids, mask []int64, err error) {
	return encodeInstructHF(path, query, passage, maxInstructLen)
}

// isInstructModel is true for decoder rerankers that score a chat template
// (zerank-1-small, Qwen3-Reranker) rather than XLM-R pair ids (BGE).
func isInstructModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "zerank") || strings.Contains(n, "qwen")
}

func instructChat(query, passage string) (prefix, doc, suffix string) {
	if len(query) > 2000 {
		query = query[:2000]
	}
	if len(passage) > 10000 {
		passage = passage[:10000]
	}
	prefix = "<|im_start|>system\n" + query + "<|im_end|>\n<|im_start|>user\n"
	suffix = "<|im_end|>\n<|im_start|>assistant\n"
	return prefix, passage, suffix
}
