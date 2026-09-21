//go:build hosted

package guard

import (
	"bytes"
	"io"
	"net/http"
)

// Guard is the dual-lane check on paid endpoints. Static holds operator
// managed keys (KLIMAT_API_KEYS) and wins over the paid lanes.
type Guard struct {
	AllowAll bool
	Static   Lane
	Unkey    Lane
	MPP      Lane
}

func drainBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(b))
	return b
}
