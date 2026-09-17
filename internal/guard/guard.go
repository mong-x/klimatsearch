package guard

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mong-x/klimatsearch/internal/config"
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

// New builds a Guard from process config.
// No Unkey and no MPP secret → AllowAll.
// Either set → fail-closed.
func New(cfg config.Config) (*Guard, error) {
	unkeyKey := strings.TrimSpace(cfg.UnkeyRootKey)
	mppSecret := strings.TrimSpace(cfg.MPPSecretKey)
	if unkeyKey == "" && mppSecret == "" {
		return &Guard{AllowAll: true}, nil
	}
	g := &Guard{}
	if unkeyKey != "" {
		g.Unkey = newUnkeyLane(unkeyKey)
	}
	if mppSecret != "" {
		lane, err := newMPPLane(cfg)
		if err != nil {
			return nil, err
		}
		g.MPP = lane
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
