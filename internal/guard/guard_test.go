package guard

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mong-x/klimatsearch/internal/config"
)

func TestNewAllowAllWhenSecretsUnset(t *testing.T) {
	g, err := New(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !g.AllowAll {
		t.Fatal("expected AllowAll")
	}
}

func TestOSSIgnoresBillingEnv(t *testing.T) {
	var cfg config.Config
	if cfg.Hosted() {
		t.Skip("hosted build")
	}
	t.Setenv("UNKEY_ROOT_KEY", "unkey_test")
	t.Setenv("MPP_SECRET_KEY", "secret")
	g, err := New(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !g.AllowAll {
		t.Fatal("self-hosted build must ignore Unkey/MPP env")
	}
}

func TestWrapAllowAll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	g, err := New(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/api/search")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func newKeyTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("POST /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>portal</html>"))
	})
	for _, p := range []string{"/healthz", "/openapi.yaml", "/docs"} {
		mux.HandleFunc("GET "+p, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}
	g, err := New(config.Config{APIKeys: []string{"key-one", "key-two"}})
	if err != nil {
		t.Fatal(err)
	}
	if g.AllowAll {
		t.Fatal("keys set: AllowAll must be false")
	}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	return srv
}

func TestStaticKeysNoKeyIs401(t *testing.T) {
	srv := newKeyTestServer(t)
	for _, path := range []string{"/api/search", "/"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("GET %s without key: got %d, want 401", path, resp.StatusCode)
		}
	}
}

func TestStaticKeysWrongKeyIs401(t *testing.T) {
	srv := newKeyTestServer(t)
	req, _ := http.NewRequest("GET", srv.URL+"/api/search", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("wrong key: got %d, want 401", resp.StatusCode)
	}
}

func TestStaticKeysBearerPasses(t *testing.T) {
	srv := newKeyTestServer(t)
	for _, key := range []string{"key-one", "key-two"} {
		req, _ := http.NewRequest("GET", srv.URL+"/api/search", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("Bearer %s: got %d, want 200", key, resp.StatusCode)
		}
	}
}

func TestStaticKeysHeaderAndQueryPass(t *testing.T) {
	srv := newKeyTestServer(t)
	req, _ := http.NewRequest("GET", srv.URL+"/api/search", nil)
	req.Header.Set("X-Api-Key", "key-one")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("X-Api-Key: got %d, want 200", resp.StatusCode)
	}
	resp, err = http.Get(srv.URL + "/?api_key=key-two")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("api_key query: got %d, want 200", resp.StatusCode)
	}
}

func TestStaticKeysPublicPathsStayOpen(t *testing.T) {
	srv := newKeyTestServer(t)
	for _, path := range []string{"/healthz", "/openapi.yaml", "/docs"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("public %s: got %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestStaticKeyCookieSetFromQuery(t *testing.T) {
	srv := newKeyTestServer(t)
	jar := &testJar{cookies: map[string]string{}}
	client := &http.Client{Jar: jar}
	resp, err := client.Get(srv.URL + "/api/search?api_key=key-one")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("query key: got %d", resp.StatusCode)
	}
	sc := resp.Header.Get("Set-Cookie")
	if !strings.Contains(sc, "klimat_api_key=key-one") || !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Lax") {
		t.Fatalf("Set-Cookie=%q", sc)
	}
}

func TestStaticKeyCookiePasses(t *testing.T) {
	srv := newKeyTestServer(t)
	req, _ := http.NewRequest("GET", srv.URL+"/api/search", nil)
	req.AddCookie(&http.Cookie{Name: "klimat_api_key", Value: "key-two"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("cookie key: got %d", resp.StatusCode)
	}
	// A header hit must not mint a cookie.
	if sc := resp.Header.Get("Set-Cookie"); sc != "" {
		t.Fatalf("header hit set a cookie: %q", sc)
	}
}

func TestStaticKeyWrongCookieIs401(t *testing.T) {
	srv := newKeyTestServer(t)
	req, _ := http.NewRequest("GET", srv.URL+"/api/search", nil)
	req.AddCookie(&http.Cookie{Name: "klimat_api_key", Value: "wrong"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("wrong cookie: got %d", resp.StatusCode)
	}
}

func TestStaticKeyQueryOnPostSetsCookie(t *testing.T) {
	srv := newKeyTestServer(t)
	resp, err := http.PostForm(srv.URL+"/api/search?key=1&api_key=key-one", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST with query key: got %d", resp.StatusCode)
	}
	if sc := resp.Header.Get("Set-Cookie"); !strings.Contains(sc, "klimat_api_key=key-one") {
		t.Fatalf("POST must exchange the query key for a cookie: %q", sc)
	}
}

func TestStaticKeysPublicPathsSetNoCookie(t *testing.T) {
	srv := newKeyTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz?api_key=key-one")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz: got %d", resp.StatusCode)
	}
	if sc := resp.Header.Get("Set-Cookie"); sc != "" {
		t.Fatalf("public path must not set a cookie: %q", sc)
	}
}

type testJar struct{ cookies map[string]string }

func (j *testJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _, c := range cookies {
		j.cookies[c.Name] = c.Value
	}
}
func (j *testJar) Cookies(u *url.URL) []*http.Cookie {
	var out []*http.Cookie
	for k, v := range j.cookies {
		out = append(out, &http.Cookie{Name: k, Value: v})
	}
	return out
}

// TestStaticKeyStaleCookieRecoveredByQueryLink pins the rotation recovery:
// a browser holding a stale cookie must regain access (and a fresh cookie)
// from a pasted ?api_key= link, and an invalid Bearer must not lock out a
// valid X-Api-Key.
func TestStaticKeyStaleCookieRecoveredByQueryLink(t *testing.T) {
	srv := newKeyTestServer(t)
	req, _ := http.NewRequest("GET", srv.URL+"/api/search?api_key=key-one", nil)
	req.AddCookie(&http.Cookie{Name: "klimat_api_key", Value: "rotated-away"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("stale cookie + valid query key: got %d, want 200 (paste must recover)", resp.StatusCode)
	}
	if sc := resp.Header.Get("Set-Cookie"); !strings.Contains(sc, "klimat_api_key=key-one") {
		t.Fatalf("stale cookie must be replaced: %q", sc)
	}

	req, _ = http.NewRequest("GET", srv.URL+"/api/search", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	req.Header.Set("X-Api-Key", "key-two")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("invalid Bearer + valid X-Api-Key: got %d, want 200", resp.StatusCode)
	}
}
