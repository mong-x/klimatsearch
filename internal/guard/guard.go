package guard

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Lane verifies one paid path (MPP Payment or Unkey Bearer).
type Lane interface {
	Present(*http.Request) bool
	Check(*http.Request) Decision
}

// Decision is a lane's allow/deny. Body is written when OK is false.
type Decision struct {
	OK     bool
	Status int
	Header http.Header
	Body   []byte
}

// Guard is the dual-lane check on paid endpoints.
type Guard struct {
	AllowAll bool
	Unkey    Lane
	MPP      Lane
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	return tok, tok != ""
}

func deny(status int, msg string) Decision {
	b, _ := json.Marshal(map[string]string{"error": msg})
	h := make(http.Header)
	h.Set("Content-Type", "application/json; charset=utf-8")
	return Decision{Status: status, Header: h, Body: b}
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
