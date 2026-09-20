package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/ingest"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/reranker"
	"github.com/mong-x/klimatsearch/internal/search"
	"github.com/mong-x/klimatsearch/internal/store"
	"github.com/mong-x/klimatsearch/internal/web"
)

func TestOperatorUI(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "ui.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	a := model.Resource{
		CatalogID:     model.CatalogBoverket,
		ResourceID:    "6000000991",
		NameSV:        "Betong",
		NameEN:        "Concrete",
		DescriptionSV: "Generisk betong",
		A1A3:          0.12,
		Unit:          "kg",
		Category:      "Betong",
		CategoryCode:  "6",
		Version:       "t",
		Details: model.Details{
			A1A3Conservative: 1.25,
			A4:               ptr(0.04),
			A51:              ptr(0.02),
		},
	}
	b := model.Resource{
		CatalogID:  model.CatalogBoverket,
		ResourceID: "6000000992",
		NameSV:     "Konstruktionsstål",
		NameEN:     "Structural steel",
		A1A3:       1.55,
		Unit:       "kg",
		Version:    "t",
	}
	for _, r := range []model.Resource{a, b} {
		hsh, _ := r.ContentHash()
		r.Hash = hsh
		vec, _ := fake.Embed(r.EmbeddingText())
		if err := st.Upsert(t.Context(), r, vec); err != nil {
			t.Fatal(err)
		}
	}
	eng := search.New(st, st, fake, reranker.None{})
	mux := http.NewServeMux()
	web.New(eng, st, "Boverket Klimatdatabas", web.Status{
		Tools:    []string{"search_climate_data", "get_resource_details", "compare_resources"},
		Embedder: "fake",
		Reranker: "none",
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("home empty", func(t *testing.T) {
		body, code, ct := get(t, srv.URL+"/")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(ct, "text/html") {
			t.Fatalf("content-type=%s", ct)
		}
		for _, want := range []string{"klimatsearch", `id="q"`, "spånskiva", "/mcp", "search_climate_data", "Boverket Klimatdatabas"} {
			if !strings.Contains(body, want) {
				t.Fatalf("home missing %q", want)
			}
		}
	})
	t.Run("search hits", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/?q=Betong")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, "Betong") || !strings.Contains(body, "6000000991") {
			t.Fatal("missing hit")
		}
		if !strings.Contains(body, "0.12") {
			t.Fatal("missing a1a3")
		}
		if !strings.Contains(body, "/api/search?") || !strings.Contains(body, "search_climate_data") {
			t.Fatal("missing inspector")
		}
	})
	t.Run("bad lang", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/?q=Betong&lang=de")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, "role=\"alert\"") {
			t.Fatal("expected error banner")
		}
	})
	t.Run("resource", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/resources/boverket:6000000991")
		if code != 200 {
			t.Fatalf("status=%d body=%s", code, body)
		}
		if !strings.Contains(body, "Betong") {
			t.Fatal("missing name")
		}
		if !strings.Contains(body, "klimatdatabasen.boverket.se/detaljer/6/6000000991") {
			t.Fatal("missing origin")
		}
		if !strings.Contains(body, "get_resource_details") {
			t.Fatal("missing MCP hint")
		}
	})
	t.Run("resource 404", func(t *testing.T) {
		_, code, _ := get(t, srv.URL+"/resources/nope")
		if code != 404 {
			t.Fatalf("status=%d", code)
		}
	})
	t.Run("compare form", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/compare")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, `name="a"`) {
			t.Fatal("missing compare form")
		}
	})
	t.Run("compare result", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/compare?a=6000000991&b=6000000992")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, "Betong") || !strings.Contains(body, "Konstruktionsstål") {
			t.Fatal("missing pair")
		}
		if !strings.Contains(body, "compare_resources") {
			t.Fatal("missing MCP hint")
		}
	})
	t.Run("connect", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/connect")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, "mcpServers") || !strings.Contains(body, "/mcp") {
			t.Fatal("missing MCP snippet")
		}
	})
	t.Run("data inspect", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/data")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		for _, want := range []string{"resources", "resource_vec", "resources_fts", "meta", "6000000991"} {
			if !strings.Contains(body, want) {
				t.Fatalf("data missing %q", want)
			}
		}
		emb, code, _ := get(t, srv.URL+"/data/embed/boverket:6000000991")
		if code != 200 {
			t.Fatalf("embed=%d %s", code, emb)
		}
		if !strings.Contains(emb, "L2 norm") || !strings.Contains(emb, "Embedded text") {
			t.Fatal("missing embedding view")
		}
	})
	t.Run("webhooks page", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/webhooks")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, "Webhooks") || !strings.Contains(body, `name="url"`) {
			t.Fatal("missing webhooks form")
		}
	})
	t.Run("ingest form", func(t *testing.T) {
		body, code, _ := get(t, srv.URL+"/ingest")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(body, `name="file"`) || !strings.Contains(body, "dkbr") {
			t.Fatal("missing ingest form")
		}
	})
	t.Run("static css", func(t *testing.T) {
		body, code, ct := get(t, srv.URL+"/static/app.css")
		if code != 200 {
			t.Fatalf("status=%d", code)
		}
		if !strings.Contains(ct, "text/css") && !strings.Contains(ct, "css") {
			t.Fatalf("content-type=%s", ct)
		}
		if !strings.Contains(body, "--accent") {
			t.Fatal("missing tokens")
		}
	})
}

func TestOperatorIngestBR25(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "ingest-ui.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	eng := search.New(st, st, fake, reranker.None{})
	mux := http.NewServeMux()
	ui := web.New(eng, st, "Boverket Klimatdatabas", web.Status{Embedder: "fake"})
	ui.Runner = &ingest.Runner{Store: st, Embedder: fake}
	ui.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("catalog", "dkbr")
	_ = mw.WriteField("version", "BR25")
	fw, err := mw.CreateFormFile("file", "br25.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("id,name,a1a3,unit,category\nbr-1,BR25-beton-test,0.11,kg,Beton\n"))
	_ = mw.Close()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/ingest", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	html := string(body)
	if !strings.Contains(html, "Upserted") || !strings.Contains(html, "dkbr") {
		t.Fatalf("missing result: %s", html)
	}
	resBody, code, _ := get(t, srv.URL+"/resources/dkbr:br-1")
	if code != 200 {
		t.Fatalf("resource=%d", code)
	}
	if !strings.Contains(resBody, "BR25-beton-test") {
		t.Fatal("missing ingested resource")
	}
	hits, code, _ := get(t, srv.URL+"/?q=BR25-beton-test")
	if code != 200 {
		t.Fatalf("search=%d", code)
	}
	if !strings.Contains(hits, "BR25-beton-test") {
		t.Fatal("search missed BR25 ingest")
	}

	var pbuf bytes.Buffer
	pw := multipart.NewWriter(&pbuf)
	_ = pw.WriteField("catalog", "dkbr")
	_ = pw.WriteField("action", "preview")
	pf, err := pw.CreateFormFile("file", "p.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pf.Write([]byte("uid,foo,gwp\nx2,PreviewRow,0.5\n"))
	_ = pw.Close()
	preq, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest", &pbuf)
	preq.Header.Set("Content-Type", pw.FormDataContentType())
	presp, err := http.DefaultClient.Do(preq)
	if err != nil {
		t.Fatal(err)
	}
	pbody, _ := io.ReadAll(presp.Body)
	_ = presp.Body.Close()
	htmlp := string(pbody)
	i := strings.Index(htmlp, `name="preview_id"`)
	if i < 0 {
		t.Fatalf("no preview_id: %s", htmlp)
	}
	rest := htmlp[i:]
	val := strings.Index(rest, `value="`)
	if val < 0 {
		t.Fatal("preview_id value")
	}
	rest = rest[val+len(`value="`):]
	end := strings.Index(rest, `"`)
	pid := rest[:end]
	var abuf bytes.Buffer
	aw := multipart.NewWriter(&abuf)
	_ = aw.WriteField("catalog", "dkbr")
	_ = aw.WriteField("action", "apply")
	_ = aw.WriteField("preview_id", pid)
	_ = aw.WriteField("map.id", "uid")
	_ = aw.WriteField("map.name", "foo")
	_ = aw.WriteField("map.a1a3", "gwp")
	_ = aw.Close()
	areq, _ := http.NewRequest(http.MethodPost, srv.URL+"/ingest", &abuf)
	areq.Header.Set("Content-Type", aw.FormDataContentType())
	aresp, err := http.DefaultClient.Do(areq)
	if err != nil {
		t.Fatal(err)
	}
	abody, _ := io.ReadAll(aresp.Body)
	_ = aresp.Body.Close()
	if aresp.StatusCode != 200 || !strings.Contains(string(abody), "Upserted") {
		t.Fatalf("apply=%d %s", aresp.StatusCode, abody)
	}
	got, code, _ := get(t, srv.URL+"/resources/dkbr:x2")
	if code != 200 || !strings.Contains(got, "PreviewRow") {
		t.Fatalf("stashed apply missing: %d %s", code, got)
	}
}

func get(t *testing.T, url string) (string, int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), resp.StatusCode, resp.Header.Get("Content-Type")
}

func ptr(v float64) *float64 { return &v }
