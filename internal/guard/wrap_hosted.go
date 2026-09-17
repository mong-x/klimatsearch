//go:build hosted

package guard

import (
	"encoding/json"
	"net/http"
)

// Wrap protects /api/*, /mcp, /mcp/sse, /mcp/messages, /admin/*.
// /healthz, /openapi.yaml, and /docs are public.
func (g *Guard) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if g == nil || g.AllowAll {
			next.ServeHTTP(w, r)
			return
		}
		if g.MPP != nil && g.MPP.Present(r) {
			passOrDeny(w, r, g.MPP.Check(r), next)
			return
		}
		if g.Unkey != nil && g.Unkey.Present(r) {
			passOrDeny(w, r, g.Unkey.Check(r), next)
			return
		}
		if g.MPP != nil {
			writeDecision(w, g.MPP.Check(r))
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	})
}

func publicPath(path string) bool {
	switch path {
	case "/healthz", "/openapi.yaml", "/docs":
		return true
	default:
		return false
	}
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
