package guard

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Admin gates operator mutations (ingest, webhooks, /admin/*) behind
// KLIMAT_ADMIN_TOKEN: the X-Admin-Token header or a "token" form field.
// An empty token disables the check. Compare is constant-time.
type Admin struct{ Token string }

// Required reports whether a token is configured.
func (a Admin) Required() bool { return a.Token != "" }

// OK reports whether the request carries the admin token. FormValue on a
// JSON body returns "" (same as the handlers this replaces), so headerless
// JSON callers must use X-Admin-Token.
func (a Admin) OK(r *http.Request) bool {
	if a.Token == "" {
		return true
	}
	tok := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if tok == "" {
		tok = strings.TrimSpace(r.FormValue("token"))
	}
	return subtle.ConstantTimeCompare([]byte(tok), []byte(a.Token)) == 1
}
