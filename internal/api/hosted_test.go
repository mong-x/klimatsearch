//go:build hosted

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mong-x/klimatsearch/internal/api"
	"github.com/mong-x/klimatsearch/internal/guard"
	"github.com/mong-x/klimatsearch/internal/testworld"
)

func TestFailClosedGuard(t *testing.T) {
	st, fake := testworld.Open(t)
	eng := testworld.Engine(st, fake)
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
