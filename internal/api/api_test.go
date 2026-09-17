package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mong-x/klimatsearch/internal/api"
	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
)

func TestHTTP(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	a := model.Resource{ResourceID: "6000000991", NameSV: "Betong", NameEN: "Concrete", A1A3: 0.12, Unit: "kg", Version: "t"}
	b := model.Resource{ResourceID: "6000000992", NameSV: "Konstruktionsstål", NameEN: "Structural steel", A1A3: 1.55, Unit: "kg", Version: "t"}
	for _, r := range []model.Resource{a, b} {
		h, _ := r.ContentHash()
		r.Hash = h
		vec, _ := fake.Embed(r.EmbeddingText())
		if err := st.Upsert(t.Context(), r, vec); err != nil {
			t.Fatal(err)
		}
	}
	eng := search.New(st, st, fake, reranker.None{})
	mux := http.NewServeMux()
	api.New(eng, st, "Boverket Klimatdatabas").Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("healthz", func(t *testing.T) {
		resp := get(t, srv.URL+"/healthz")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
	})
	t.Run("empty q 400", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/search?q=")
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("got %d", resp.StatusCode)
		}
	})
	t.Run("unknown lang 400", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/search?q=betong&lang=de")
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("got %d", resp.StatusCode)
		}
	})
	t.Run("lang default sv", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/search?q=Betong")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["lang"] != "sv" {
			t.Fatalf("lang=%v", body["lang"])
		}
		if body["source"] != "Boverket Klimatdatabas" {
			t.Fatalf("source=%v", body["source"])
		}
	})
	t.Run("vector true 200", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/search?q=Betong&vector=true")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var body struct {
			Results []struct {
				ID          string  `json:"id"`
				MatchSource string  `json:"match_source"`
				Score       float64 `json:"score"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Results) == 0 {
			t.Fatal("expected hits")
		}
	})
	t.Run("list and get", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/resources?lang=sv")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var body struct {
			Resources []struct {
				ID string `json:"id"`
			} `json:"resources"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Resources) < 1 {
			t.Fatal("empty list")
		}
		id := body.Resources[0].ID
		g := get(t, srv.URL+"/api/resources/"+id+"?lang=sv")
		defer g.Body.Close()
		if g.StatusCode != 200 {
			t.Fatal(g.Status)
		}
		missing := get(t, srv.URL+"/api/resources/nope")
		defer missing.Body.Close()
		if missing.StatusCode != 404 {
			t.Fatalf("want 404 got %d", missing.StatusCode)
		}
	})
	t.Run("compare", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/resources/compare?a=6000000991&b=6000000992")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["lower_impact_id"] != "6000000991" {
			t.Fatalf("%v", body)
		}
		if body["incomparable"] != false {
			t.Fatalf("incomparable=%v", body["incomparable"])
		}

		missing := get(t, srv.URL+"/api/resources/compare?a=6000000991&b=missing")
		defer missing.Body.Close()
		if missing.StatusCode != 404 {
			t.Fatalf("want 404 got %d", missing.StatusCode)
		}

		bad := get(t, srv.URL+"/api/resources/compare?a=6000000991&b=6000000992&unit=m2")
		defer bad.Body.Close()
		if bad.StatusCode != 400 {
			t.Fatalf("want 400 got %d", bad.StatusCode)
		}
	})
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
