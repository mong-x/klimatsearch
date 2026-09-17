package guard

import (
	"net/http"
	"net/http/httptest"
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

func TestNewFailClosedWhenMPPSet(t *testing.T) {
	g, err := New(config.Config{
		MPPSecretKey: "test-secret",
		MPPRecipient: "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.AllowAll {
		t.Fatal("expected fail-closed")
	}
	if g.MPP == nil {
		t.Fatal("expected MPP lane")
	}
}

func TestNewMPPRequiresRecipient(t *testing.T) {
	_, err := New(config.Config{MPPSecretKey: "test-secret"})
	if err == nil {
		t.Fatal("expected error when MPP_RECIPIENT is empty")
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

func TestMPPBeforeUnkey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	g := &Guard{
		MPP:   fakeLane{present: true, ok: true},
		Unkey: fakeLane{present: true, ok: false},
	}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/api/search")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("got %d, MPP should win over Unkey", resp.StatusCode)
	}
}

func TestUnkeyBearerWhenMPPNotPresent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	g := &Guard{
		MPP:   fakeLane{present: false, ok: false},
		Unkey: fakeLane{present: true, ok: true},
	}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/search", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer sk_test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("got %d, Unkey Bearer should pass when MPP is not present", resp.StatusCode)
	}
}

func TestWrapMPP402WithoutPayment(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	g, err := New(config.Config{
		MPPSecretKey: "test-secret",
		MPPRecipient: "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
	})
	if err != nil {
		t.Fatal(err)
	}
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
	if denied.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("search=%d want 402", denied.StatusCode)
	}
	auth := denied.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(strings.ToLower(auth), "payment ") {
		t.Fatalf("WWW-Authenticate=%q, want Payment challenge", auth)
	}
}
