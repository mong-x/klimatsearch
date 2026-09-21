//go:build !hosted

package guard

import "net/http"

// Wrap is a pass-through in the self-hosted build unless the operator set
// KLIMAT_API_KEYS, in which case every non-public path (web portal, REST,
// MCP) requires one of those keys as Authorization Bearer, X-Api-Key, or
// api_key query parameter.
func (g *Guard) Wrap(next http.Handler) http.Handler {
	if g == nil || g.Static == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		passOrDeny(w, r, g.Static.Check(r), next)
	})
}
