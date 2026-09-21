//go:build hosted

package guard

import (
	"net/http"
)

// Wrap protects /api/*, /mcp, /mcp/sse, /mcp/messages, /admin/* and the web
// portal. /healthz, /openapi.yaml, and /docs are public. Static operator keys
// (KLIMAT_API_KEYS) are checked first, then MPP, then Unkey.
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
		if g.Static != nil {
			passOrDeny(w, r, g.Static.Check(r), next)
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
