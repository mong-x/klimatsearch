package reranker

const maxPairLen = 512

func encodePair(path, query, passage string) (ids, mask []int64, err error) {
	return encodePairHF(path, query, passage, maxPairLen)
}
