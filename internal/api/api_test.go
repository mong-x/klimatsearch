package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mong-x/klimatsearch/internal/api"
	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/guard"
	"github.com/mong-x/klimatsearch/internal/ingest"
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
	a := model.Resource{
		CatalogID:       model.CatalogBoverket,
		ResourceID:      "6000000991",
		NameSV:          "Betong",
		NameEN:          "Concrete",
		DescriptionSV:   "Generisk betong",
		ApplicabilitySV: "Stomme",
		A1A3:            0.12,
		Unit:            "kg",
		Conversions:     map[string]float64{"kg/m³": 2400},
		Category:        "Betong",
		CategoryCode:    "6",
		Version:         "t",
	}
	b := model.Resource{CatalogID: model.CatalogBoverket, ResourceID: "6000000992", NameSV: "Konstruktionsstål", NameEN: "Structural steel", A1A3: 1.55, Unit: "kg", Version: "t"}
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
	h := api.New(eng, st, "Boverket Klimatdatabas")
	h.Runner = &ingest.Runner{Store: st, Embedder: fake}
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("healthz", func(t *testing.T) {
		resp := get(t, srv.URL+"/healthz")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
	})
	t.Run("openapi yaml", func(t *testing.T) {
		resp := get(t, srv.URL+"/openapi.yaml")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("openapi: 3.0.3")) {
			t.Fatal("missing openapi version")
		}
		if !bytes.Contains(buf.Bytes(), []byte("/api/resources/{id}/origin")) {
			t.Fatal("missing origin path")
		}
	})
	t.Run("docs swagger ui", func(t *testing.T) {
		resp := get(t, srv.URL+"/docs")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("content-type=%s", ct)
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
	t.Run("search hit includes full resource and details", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/search?q=Betong")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.Status)
		}
		var body struct {
			Results []map[string]any `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Results) == 0 {
			t.Fatal("expected hits")
		}
		hit := body.Results[0]
		if hit["id"] != "6000000991" {
			t.Fatalf("id=%v", hit["id"])
		}
		if hit["category"] != "Betong" {
			t.Fatalf("category=%v", hit["category"])
		}
		if hit["applicability_sv"] != "Stomme" {
			t.Fatalf("applicability_sv=%v", hit["applicability_sv"])
		}
		conv, _ := hit["conversions"].(map[string]any)
		if conv["kg/m³"] != 2400.0 {
			t.Fatalf("conversions=%v", hit["conversions"])
		}
		details, _ := hit["details"].(string)
		if details != "/api/resources/boverket:6000000991" {
			t.Fatalf("details=%q", details)
		}
		g := get(t, srv.URL+details)
		defer g.Body.Close()
		if g.StatusCode != 200 {
			t.Fatalf("GET %s want 200 got %d", details, g.StatusCode)
		}
		var got map[string]any
		if err := json.NewDecoder(g.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got["id"] != "6000000991" || got["category"] != "Betong" {
			t.Fatalf("details body=%v", got)
		}
		if got["origin"] != "https://klimatdatabasen.boverket.se/detaljer/6/6000000991" {
			t.Fatalf("origin=%v", got["origin"])
		}
		noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		redir, err := noFollow.Get(srv.URL + "/api/resources/boverket:6000000991/origin")
		if err != nil {
			t.Fatal(err)
		}
		defer redir.Body.Close()
		if redir.StatusCode != http.StatusFound {
			t.Fatalf("origin redirect=%d", redir.StatusCode)
		}
		if loc := redir.Header.Get("Location"); loc != "https://klimatdatabasen.boverket.se/detaljer/6/6000000991" {
			t.Fatalf("Location=%q", loc)
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
	t.Run("list catalog field", func(t *testing.T) {
		resp := get(t, srv.URL+"/api/resources")
		defer resp.Body.Close()
		var body struct {
			Resources []struct {
				ID      string `json:"id"`
				Catalog string `json:"catalog"`
				Source  string `json:"source"`
			} `json:"resources"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Resources) == 0 || body.Resources[0].Catalog != "boverket" {
			t.Fatalf("%+v", body.Resources)
		}
		if body.Resources[0].Source != "Boverket Klimatdatabas" {
			t.Fatalf("source=%s", body.Resources[0].Source)
		}
	})
	t.Run("admin ingest missing file 400", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/ingest/file?catalog=br25", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("want 400 got %d", resp.StatusCode)
		}
	})
	t.Run("admin ingest unknown catalog 400", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", "x.csv")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte("id,name\n1,x\n"))
		_ = mw.Close()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/admin/ingest/file?catalog=unknown", &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatalf("want 400 got %d", resp.StatusCode)
		}
	})
}

func TestAmbiguousResourceHTTP(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "amb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	a := model.Resource{CatalogID: "boverket", ResourceID: "same", NameSV: "A", A1A3: 0.1, Unit: "kg"}
	b := model.Resource{CatalogID: "br25", ResourceID: "same", NameSV: "B", A1A3: 0.2, Unit: "kg"}
	for _, r := range []model.Resource{a, b} {
		hsh, _ := r.ContentHash()
		r.Hash = hsh
		if err := st.Upsert(t.Context(), r, nil); err != nil {
			t.Fatal(err)
		}
	}
	eng := search.New(st, st, fake, reranker.None{})
	mux := http.NewServeMux()
	h := api.New(eng, st, "Boverket Klimatdatabas")
	h.Runner = &ingest.Runner{Store: st, Embedder: fake}
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp := get(t, srv.URL+"/api/resources/same")
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("want 409 got %d", resp.StatusCode)
	}
	ok := get(t, srv.URL+"/api/resources/br25:same")
	defer ok.Body.Close()
	if ok.StatusCode != 200 {
		t.Fatal(ok.Status)
	}
	list := get(t, srv.URL+"/api/resources?databases=br25")
	defer list.Body.Close()
	var body struct {
		Resources []struct {
			Catalog string `json:"catalog"`
		} `json:"resources"`
	}
	if err := json.NewDecoder(list.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Resources) != 1 || body.Resources[0].Catalog != "br25" {
		t.Fatalf("%+v", body.Resources)
	}
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAdminTokenRequired(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	h := api.New(nil, st, "Boverket Klimatdatabas")
	h.Runner = &ingest.Runner{Store: st, Embedder: fake}
	h.Admin = guard.Admin{Token: "t0k"}
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	payload := `{"catalog":"boverket","resources":[{"id":"x1","name":"X","unit":"kg","a1a3":0.1}]}`
	post := func(contentType, body, token string) int {
		req, _ := http.NewRequest("POST", srv.URL+"/admin/resources", strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if token != "" {
			req.Header.Set("X-Admin-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	if got := post("application/json", payload, ""); got != 401 {
		t.Fatalf("no token: got %d", got)
	}
	if got := post("application/json", payload, "wrong"); got != 401 {
		t.Fatalf("wrong token: got %d", got)
	}
	if got := post("application/json", payload, "t0k"); got != 200 {
		t.Fatalf("header token: got %d", got)
	}
	// The form-field idiom is console-only; this endpoint takes JSON bodies,
	// where FormValue cannot read a token (unchanged from the old adminOK).
}

// TestAdminResourcesBatchConversions pins that a batch upsert can carry
// Conversions and the Comparison impacts, and that GET returns them.
func TestAdminResourcesBatchConversions(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "batch-conv.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	h := api.New(nil, st, "Boverket Klimatdatabas")
	h.Runner = &ingest.Runner{Store: st, Embedder: fake}
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	payload := `{"catalog":"boverket","resources":[{"id":"c1","name":"Betong","unit":"kg","a1a3":0.12,` +
		`"conversions":{"kg/m³":2400},"a4":0.04,"a5_1":0.02,"a1a3_conservative":0.15}]}`
	resp, err := http.Post(srv.URL+"/admin/resources", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("upsert: %d %s", resp.StatusCode, body)
	}

	gresp := get(t, srv.URL+"/api/resources/boverket:c1")
	gbody, _ := io.ReadAll(gresp.Body)
	_ = gresp.Body.Close()
	res := string(gbody)
	if gresp.StatusCode != 200 {
		t.Fatalf("get: %d %s", gresp.StatusCode, res)
	}
	if !strings.Contains(res, `"kg/m³":2400`) {
		t.Fatalf("conversions missing from view: %s", res)
	}
	for _, want := range []string{`"a4"`, `"a5_1"`, `"a1a3_conservative"`} {
		if !strings.Contains(res, want) {
			t.Fatalf("%s missing from view: %s", want, res)
		}
	}
}
