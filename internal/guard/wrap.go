//go:build !hosted

package guard

import "net/http"

// Wrap is a pass-through in the self-hosted build.
func (g *Guard) Wrap(next http.Handler) http.Handler {
	return next
}
