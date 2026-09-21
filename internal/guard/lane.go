package guard

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

// Lane verifies one paid path (MPP Payment, Unkey Bearer, or static key).
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

// publicPath lists paths that never require a key: health, specs, docs.
func publicPath(path string) bool {
	switch path {
	case "/healthz", "/openapi.yaml", "/docs":
		return true
	default:
		return false
	}
}

// staticLane gates requests behind operator-managed static keys
// (KLIMAT_API_KEYS). It needs no external service and works in every build.
type staticLane struct {
	keys []string
}

func newStaticLane(keys []string) Lane {
	if len(keys) == 0 {
		return nil
	}
	return &staticLane{keys: keys}
}

// staticKey reads the caller's key: Authorization Bearer, X-Api-Key header,
// or api_key query parameter (so a browser can open the portal with a link).
func staticKey(r *http.Request) (string, bool) {
	if tok, ok := bearerToken(r); ok {
		return tok, true
	}
	if tok := strings.TrimSpace(r.Header.Get("X-Api-Key")); tok != "" {
		return tok, true
	}
	if tok := strings.TrimSpace(r.URL.Query().Get("api_key")); tok != "" {
		return tok, true
	}
	return "", false
}

func (s *staticLane) Present(r *http.Request) bool {
	_, ok := staticKey(r)
	return ok
}

func (s *staticLane) Check(r *http.Request) Decision {
	tok, ok := staticKey(r)
	if !ok {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	for _, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(k), []byte(tok)) == 1 {
			return Decision{OK: true}
		}
	}
	return deny(http.StatusUnauthorized, "unauthorized")
}

func passOrDeny(w http.ResponseWriter, r *http.Request, d Decision, next http.Handler) {
	if !d.OK {
		writeDecision(w, d)
		return
	}
	applyHeaders(w, d.Header)
	next.ServeHTTP(w, r)
}

func applyHeaders(w http.ResponseWriter, h http.Header) {
	if h == nil {
		return
	}
	for k, vs := range h {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
}

func writeDecision(w http.ResponseWriter, d Decision) {
	applyHeaders(w, d.Header)
	status := d.Status
	if status == 0 {
		status = http.StatusUnauthorized
	}
	if d.Body != nil {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.WriteHeader(status)
		_, _ = w.Write(d.Body)
		return
	}
	writeJSON(w, status, map[string]string{"error": "unauthorized"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
