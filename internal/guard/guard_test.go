package guard

import (
	"net/http"
	"net/http/httptest"
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

func TestNewFailClosedWhenUnkeySet(t *testing.T) {
	g, err := New(config.Config{UnkeyRootKey: "unkey_test"})
	if err != nil {
		t.Fatal(err)
	}
	if g.AllowAll {
		t.Fatal("expected fail-closed")
	}
	if g.Unkey == nil {
		t.Fatal("expected Unkey lane")
	}
}

func TestWrapAllowAll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	g := &Guard{AllowAll: true}
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

func TestWrapFailClosed401AndHealthz(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	g := &Guard{}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	hz, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer hz.Body.Close()
	if hz.StatusCode != 200 {
		t.Fatalf("healthz=%d", hz.StatusCode)
	}
	denied, err := http.Get(srv.URL + "/api/search")
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	if denied.StatusCode != 401 {
		t.Fatalf("search=%d", denied.StatusCode)
	}
}

func TestWrap402WithStorefront(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	g := &Guard{Storefront: "https://pay.example"}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	denied, err := http.Get(srv.URL + "/api/search")
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	if denied.StatusCode != 402 {
		t.Fatalf("search=%d", denied.StatusCode)
	}
}

type fakeLane struct {
	present bool
	ok      bool
}

func (f fakeLane) Present(*http.Request) bool { return f.present }
func (f fakeLane) Check(*http.Request) Decision {
	if f.ok {
		return Decision{OK: true}
	}
	return deny(http.StatusUnauthorized, "unauthorized")
}

func TestZeroClickLaneFirst(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	g := &Guard{
		ZeroClick: fakeLane{present: true, ok: true},
		Unkey:     fakeLane{present: true, ok: false},
	}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/api/search")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("got %d, ZeroClick should win", resp.StatusCode)
	}
}
