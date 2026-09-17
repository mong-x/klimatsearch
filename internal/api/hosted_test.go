//go:build hosted

package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/api"
	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/guard"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

func TestFailClosedGuard(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	eng := search.New(st, st, fake, reranker.None{})
	mux := http.NewServeMux()
	api.New(eng, st, "Boverket Klimatdatabas").Register(mux)
	g := &guard.Guard{}
	srv := httptest.NewServer(g.Wrap(mux))
	t.Cleanup(srv.Close)

	hz := get(t, srv.URL+"/healthz")
	defer hz.Body.Close()
	if hz.StatusCode != 200 {
		t.Fatalf("healthz=%d", hz.StatusCode)
	}
	denied := get(t, srv.URL+"/api/search?q=betong")
	defer denied.Body.Close()
	if denied.StatusCode != 401 {
		t.Fatalf("search without auth want 401 got %d", denied.StatusCode)
	}
}
