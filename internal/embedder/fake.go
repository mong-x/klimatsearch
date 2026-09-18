package embedder

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

const Dim = 320

// Fake is a deterministic Embedder: SHA-256 of UTF-8, repeated little-endian
// chunks expanded to Dim float32 values, then L2-normalized.
type Fake struct{}

func (Fake) Dim() int { return Dim }

func (Fake) EmbedQuery(query string) ([]float32, error) {
	return Fake{}.Embed(query)
}

func (Fake) Embed(text string) ([]float32, error) {
	sum := sha256.Sum256([]byte(text))
	raw := make([]byte, Dim*4)
	for i := 0; i < len(raw); i += len(sum) {
		copy(raw[i:], sum[:])
	}
	vec := make([]float32, Dim)
	for i := 0; i < Dim; i++ {
		u := binary.LittleEndian.Uint32(raw[i*4 : (i+1)*4])
		vec[i] = float32(int32(u))
	}
	return l2normalize(vec), nil
}

func l2normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	n := math.Sqrt(sum)
	if n == 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		out := make([]float32, len(v))
		if len(out) > 0 {
			out[0] = 1
		}
		return out
	}
	out := make([]float32, len(v))
	inv := float32(1 / n)
	for i, x := range v {
		out[i] = x * inv
	}
	return out
}
