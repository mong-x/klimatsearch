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

// Present reports whether the request carries any static credential:
// Authorization Bearer, X-Api-Key header, the klimat_api_key cookie, or the
// api_key query parameter. A browser opens the portal with a query-key link
// once and keeps running on the cookie.
func (s *staticLane) Present(r *http.Request) bool {
	return len(staticCandidates(r)) > 0
}

func (s *staticLane) Check(r *http.Request) Decision {
	// Try every presented credential, not just the first: a stale cookie
	// (e.g. after a key rotation) must not lock out a fresh ?api_key= link.
	var allowed bool
	for _, tok := range staticCandidates(r) {
		if s.match(tok) {
			allowed = true
			break
		}
	}
	if !allowed {
		return deny(http.StatusUnauthorized, "unauthorized")
	}
	d := Decision{OK: true, Header: http.Header{}}
	// Exchange a valid query key for a session cookie so console
	// navigation (links, forms, redirects) survives past the first URL —
	// browsers cannot set headers on form POSTs.
	if q := r.URL.Query().Get("api_key"); q != "" && s.match(q) {
		d.Header.Add("Set-Cookie", sessionCookie(q, r.TLS != nil))
	}
	return d
}

// staticCandidates lists every credential the request presents.
func staticCandidates(r *http.Request) []string {
	var out []string
	if tok, ok := bearerToken(r); ok {
		out = append(out, tok)
	}
	if tok := strings.TrimSpace(r.Header.Get("X-Api-Key")); tok != "" {
		out = append(out, tok)
	}
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		out = append(out, c.Value)
	}
	if tok := strings.TrimSpace(r.URL.Query().Get("api_key")); tok != "" {
		out = append(out, tok)
	}
	return out
}

func (s *staticLane) match(tok string) bool {
	for _, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(k), []byte(tok)) == 1 {
			return true
		}
	}
	return false
}

// cookieName is the static-key session cookie. SameSite=Lax is load-bearing:
// it keeps the cookie off cross-site POSTs (CSRF) while allowing top-level
// navigation. Session lifetime — the raw key dies with the browser.
const cookieName = "klimat_api_key"

func sessionCookie(v string, secure bool) string {
	c := &http.Cookie{Name: cookieName, Value: v, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode}
	if secure {
		c.Secure = true
	}
	return c.String()
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
