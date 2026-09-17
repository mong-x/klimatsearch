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
