package guard

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
)

// Lane verifies one paid path (ZeroClick signature or Unkey Bearer).
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
	AllowAll   bool
	ZeroClick  Lane
	Unkey      Lane
	Storefront string
}

// New builds a Guard from process config.
// No Unkey root key and no ZeroClick signing secrets → AllowAll.
// Either set → fail-closed.
func New(cfg config.Config) (*Guard, error) {
	unkeyKey := strings.TrimSpace(cfg.UnkeyRootKey)
	zcSecrets := strings.TrimSpace(cfg.ZeroClickSigningSecrets)
	if unkeyKey == "" && zcSecrets == "" {
		return &Guard{AllowAll: true, Storefront: cfg.ZeroClickStorefrontURL}, nil
	}
	g := &Guard{Storefront: cfg.ZeroClickStorefrontURL}
	if unkeyKey != "" {
		g.Unkey = newUnkeyLane(unkeyKey)
	}
	if zcSecrets != "" {
		lane, err := newZeroClickLane(cfg)
		if err != nil {
			return nil, err
		}
		g.ZeroClick = lane
	}
	return g, nil
}

// Wrap protects /api/*, /mcp, /mcp/sse, /mcp/messages, /admin/*. /healthz is public.
func (g *Guard) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if g == nil || g.AllowAll {
			next.ServeHTTP(w, r)
			return
		}
		if g.ZeroClick != nil && g.ZeroClick.Present(r) {
			d := g.ZeroClick.Check(r)
			if !d.OK {
				writeDecision(w, d)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if g.Unkey != nil && g.Unkey.Present(r) {
			d := g.Unkey.Check(r)
			if !d.OK {
				writeDecision(w, d)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if g.Storefront != "" {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{
				"error":      "payment required",
				"storefront": g.Storefront,
			})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	})
}

func writeDecision(w http.ResponseWriter, d Decision) {
	if d.Header != nil {
		for k, vs := range d.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
	}
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
