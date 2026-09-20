package ingest

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/mong-x/klimatsearch/internal/embedder"
	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/store"
)

func TestParseBoverketV2One(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "fixtures", "boverket-v2-one.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := ParseJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Resources) != 1 {
		t.Fatalf("got %d", len(batch.Resources))
	}
	r := batch.Resources[0]
	if r.ResourceID != "6000000000" {
		t.Fatalf("id=%s", r.ResourceID)
	}
	if r.NameSV != "Spånskiva" || r.NameEN != "Particle board" {
		t.Fatalf("names %+v", r)
	}
	if r.A1A3 != 0.39 {
		t.Fatalf("a1a3=%v (must be Typical, not Conservative 0.488)", r.A1A3)
	}
	if r.Details.A1A3Conservative != 0.488 {
		t.Fatalf("conservative=%v", r.Details.A1A3Conservative)
	}
	if r.Details.A4 == nil || *r.Details.A4 != 0.0629 {
		t.Fatalf("a4=%v", r.Details.A4)
	}
	if r.Details.A51 == nil || *r.Details.A51 != 0.055 {
		t.Fatalf("a5_1=%v", r.Details.A51)
	}
	if r.Details.BiogenicCarbon == nil || *r.Details.BiogenicCarbon != 0.42 {
		t.Fatalf("biogenic=%v", r.Details.BiogenicCarbon)
	}
	if r.Details.WasteFactor != 1.1 || r.Details.ConservativeFactor != 1.25 {
		t.Fatalf("factors waste=%v cons=%v", r.Details.WasteFactor, r.Details.ConservativeFactor)
	}
	if r.Details.ServiceLife == "" || r.Details.UseAdviceSV == "" || r.Details.CommentSV == "" {
		t.Fatalf("text details %+v", r.Details)
	}
	if r.Details.BK04Code != "01208" {
		t.Fatalf("bk04=%s", r.Details.BK04Code)
	}
	v := r.View("Boverket Klimatdatabas")
	if v["a4"] == nil || v["a5_1"] == nil || v["a1a3_conservative"] == nil {
		t.Fatalf("view missing climate modules: %+v", v)
	}
	if r.Unit != "kg" {
		t.Fatalf("unit=%s", r.Unit)
	}
	if r.Conversions["kg/m³"] != 700 {
		t.Fatalf("conv %+v", r.Conversions)
	}
	if r.Category != "Byggskivor" {
		t.Fatalf("category=%s", r.Category)
	}
	if r.CategoryCode != "10" {
		t.Fatalf("category_code=%s", r.CategoryCode)
	}
	wantOrigin := "https://klimatdatabasen.boverket.se/detaljer/10/6000000000"
	if r.Origin() != wantOrigin {
		t.Fatalf("origin=%s", r.Origin())
	}
	if r.CatalogID != "boverket" {
		t.Fatalf("catalog=%s", r.CatalogID)
	}
	if r.Synonyms == "" {
		t.Fatal("expected synonyms")
	}
	if r.ApplicabilitySV == "" {
		t.Fatal("expected applicability sv")
	}
}

func TestParseJSONFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "fixtures", "resources.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := ParseJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Resources) != 5 {
		t.Fatalf("got %d", len(batch.Resources))
	}
	var found bool
	for _, r := range batch.Resources {
		if r.NameSV == "Betong" && r.NameEN == "Concrete" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing Betong/Concrete")
	}
}

func TestHashDiffUpsert(t *testing.T) {
	if os.Getenv("CGO_ENABLED") == "0" {
		t.Skip("CGO is disabled; sqlite store tests require CGO_ENABLED=1")
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "in.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var fake embedder.Fake
	st.ConfigureVector(fake.Dim())
	r := &Runner{Store: st, Embedder: fake}
	batch := Batch{
		Origin:  "fixture",
		Version: "1",
		Resources: []model.Resource{{
			ResourceID: "1", NameSV: "Betong", NameEN: "Concrete", A1A3: 0.1, Unit: "kg",
		}},
	}
	res, err := r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 1 {
		t.Fatalf("upserted %d", res.Upserted)
	}
	res, err = r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 || res.Upserted != 0 {
		t.Fatalf("expected skip, got %+v", res)
	}
	batch.Resources[0].A1A3 = 0.2
	res, err = r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 1 {
		t.Fatalf("changed hash should upsert, got %+v", res)
	}

	batch.Resources[0].DescriptionSV = "Ny beskrivning"
	res, err = r.Apply(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 1 {
		t.Fatalf("description-only edit should upsert, got %+v", res)
	}
}

func TestExcelParseAndMerge(t *testing.T) {
	sv := writeXLSX(t, []string{
		"Resurs-ID", "Produktnamn", "Kategori", "Version",
		"Enhet för klimatpåverkan",
		"A1-A3 byggproduktens klimatpåverkan GWP-GHG, typiskt värde",
		"Omräkningsfaktor", "Enhet för omräkningsfaktor", "Teknisk beskrivning",
	}, [][]string{
		{"6000000991", "Betong", "Betong", "02.07.000", "kg CO₂e/kg", "0.12", "2400", "kg/m³", "Generisk betong"},
		{"6000000992", "Stål", "Stål", "02.07.000", "kg CO₂e/kg", "1.55", "1", "kg", "Stål"},
	})
	en := writeXLSX(t, []string{
		"Resource ID", "Product name", "Category", "Version",
		"Unit for climate impact",
		"A1-A3 building product's climate impact GWP-GHG, typical value",
		"Conversion factor", "Unit for conversion factor", "Technical description",
	}, [][]string{
		{"6000000991", "Concrete", "Concrete", "02.07.000", "kg CO₂e/kg", "0.12", "2400", "kg/m³", "Generic concrete"},
		{"6000000992", "Steel", "Steel", "02.07.000", "kg CO₂e/kg", "1.55", "1", "kg", "Steel"},
	})
	svb, err := ParseExcel(sv, "sv")
	if err != nil {
		t.Fatal(err)
	}
	enb, err := ParseExcel(en, "en")
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeLang(svb, enb)
	if len(merged.Resources) != 2 {
		t.Fatalf("merged %d", len(merged.Resources))
	}
	var betong model.Resource
	for _, r := range merged.Resources {
		if r.ResourceID == "6000000991" {
			betong = r
		}
	}
	if betong.NameSV != "Betong" || betong.NameEN != "Concrete" {
		t.Fatalf("%+v", betong)
	}
	if betong.A1A3 != 0.12 || betong.Unit != "kg" {
		t.Fatalf("a1a3/unit %+v", betong)
	}
	if betong.Conversions["kg/m³"] != 2400 {
		t.Fatalf("conv %+v", betong.Conversions)
	}
}

func TestExcelParsesDetails(t *testing.T) {
	raw := writeXLSX(t, []string{
		"Resurs-ID", "Produktnamn", "Kategori", "Version",
		"Enhet för klimatpåverkan",
		"A1-A3 byggproduktens klimatpåverkan GWP-GHG, typiskt värde",
		"A1-A3 byggproduktens klimatpåverkan GWP-GHG, konservativt värde",
		"A4", "A5.1", "Avfallsfaktor", "A1-A3 faktor för konservativa värden",
		"Omräkningsfaktor", "Enhet för omräkningsfaktor", "Teknisk beskrivning",
	}, [][]string{
		{"6000000000", "Spånskiva", "Byggskivor", "02.07.000", "kg CO₂e/kg",
			"0.39", "0.488", "0.0629", "0.055", "1.1", "1.25", "700", "kg/m³", "skiva"},
	})
	b, err := ParseExcel(raw, "sv")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Resources) != 1 {
		t.Fatalf("n=%d", len(b.Resources))
	}
	r := b.Resources[0]
	if r.A1A3 != 0.39 || r.Details.A1A3Conservative != 0.488 {
		t.Fatalf("a1a3 %+v cons %v", r.A1A3, r.Details.A1A3Conservative)
	}
	if r.Details.A4 == nil || *r.Details.A4 != 0.0629 {
		t.Fatalf("a4=%v", r.Details.A4)
	}
	if r.Details.A51 == nil || *r.Details.A51 != 0.055 {
		t.Fatalf("a5=%v", r.Details.A51)
	}
	if r.Details.WasteFactor != 1.1 || r.Details.ConservativeFactor != 1.25 {
		t.Fatalf("factors %+v", r.Details)
	}
}

func writeXLSX(t *testing.T, headers []string, rows [][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue("Sheet1", cell, h); err != nil {
			t.Fatal(err)
		}
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue("Sheet1", cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCommittedExcelFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "fixtures", "resources.xlsx")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := ParseExcel(b, "sv")
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Resources) != 5 {
		t.Fatalf("got %d", len(batch.Resources))
	}
}

func TestStaticFetcherNoNetwork(t *testing.T) {
	f := StaticFetcher{Batch: Batch{Resources: []model.Resource{{ResourceID: "1"}}, Origin: "fixture"}}
	b, err := f.Fetch(t.Context())
	if err != nil || len(b.Resources) != 1 {
		t.Fatalf("%+v %v", b, err)
	}
}

func miniExcel(t *testing.T) []byte {
	t.Helper()
	return writeXLSX(t, []string{
		"Resurs-ID", "Produktnamn", "Kategori", "Version",
		"Enhet för klimatpåverkan",
		"A1-A3 byggproduktens klimatpåverkan GWP-GHG, typiskt värde",
		"Omräkningsfaktor", "Enhet för omräkningsfaktor", "Teknisk beskrivning",
	}, [][]string{
		{"6000000991", "Betong", "Betong", "02.07.000", "kg CO₂e/kg", "0.12", "2400", "kg/m³", "Generisk betong"},
	})
}

func TestHTTPFetcherJSON404FallsBackToExcel(t *testing.T) {
	xlsx := miniExcel(t)
	var excelGets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".xlsx") {
			excelGets++
			_, _ = w.Write(xlsx)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	f := &HTTPFetcher{
		Client:  srv.Client(),
		APIBase: srv.URL,
		ExcelSV: srv.URL + "/sv.xlsx",
		ExcelEN: srv.URL + "/en.xlsx",
	}
	batch, err := f.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if excelGets == 0 {
		t.Fatal("excel path not invoked after JSON 404")
	}
	if len(batch.Resources) == 0 {
		t.Fatal("expected excel resources")
	}
}

func TestHTTPFetcherEmptyJSONFallsBackToExcel(t *testing.T) {
	xlsx := miniExcel(t)
	var excelGets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".xlsx") {
			excelGets++
			_, _ = w.Write(xlsx)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	f := &HTTPFetcher{
		Client:  srv.Client(),
		APIBase: srv.URL,
		ExcelSV: srv.URL + "/sv.xlsx",
		ExcelEN: srv.URL + "/en.xlsx",
	}
	batch, err := f.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if excelGets == 0 {
		t.Fatal("excel path not invoked after empty JSON")
	}
	if len(batch.Resources) == 0 {
		t.Fatal("expected excel resources")
	}
}

func TestHTTPFetcherZeroA1A3KeepsJSON(t *testing.T) {
	var excelGets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".xlsx") {
			excelGets++
			http.Error(w, "excel should not be fetched", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"1","name_sv":"x","a1a3":0,"unit":"kg"}]`))
	}))
	t.Cleanup(srv.Close)
	f := &HTTPFetcher{
		Client:  srv.Client(),
		APIBase: srv.URL,
		ExcelSV: srv.URL + "/sv.xlsx",
		ExcelEN: srv.URL + "/en.xlsx",
	}
	batch, err := f.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if excelGets != 0 {
		t.Fatal("A1A3==0 must not fall back to excel")
	}
	if len(batch.Resources) != 1 || batch.Resources[0].A1A3 != 0 {
		t.Fatalf("%+v", batch.Resources)
	}
	if batch.Origin != "json" {
		t.Fatalf("origin=%s", batch.Origin)
	}
}

func TestHTTPFetcherUsesV2Paths(t *testing.T) {
	v2, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "boverket-v2-one.json"))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "/klimatdatabas/v1/") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "GetAllResources") && strings.HasSuffix(r.URL.Path, "/json") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(v2)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	f := &HTTPFetcher{
		Client:  srv.Client(),
		APIBase: srv.URL,
		ExcelSV: srv.URL + "/sv.xlsx",
		ExcelEN: srv.URL + "/en.xlsx",
	}
	batch, err := f.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, " ")
	if strings.Contains(joined, "/klimatdatabas/v1/resources") {
		t.Fatalf("must not probe fake v1 paths: %v", paths)
	}
	if !strings.Contains(joined, "/api/Klimat/v2/GetAllResources/latest/sv/json") {
		t.Fatalf("missing sv v2 path: %v", paths)
	}
	if !strings.Contains(joined, "/api/Klimat/v2/GetAllResources/latest/en/json") {
		t.Fatalf("missing en v2 path: %v", paths)
	}
	if len(batch.Resources) != 1 || batch.Resources[0].ResourceID != "6000000000" {
		t.Fatalf("%+v", batch.Resources)
	}
	if batch.Resources[0].A1A3 != 0.39 {
		t.Fatalf("a1a3=%v", batch.Resources[0].A1A3)
	}
}

func TestFileIngesterUnknownAndBR25(t *testing.T) {
	f := FileIngester{Catalog: "nope", Data: []byte("x")}
	_, err := f.Fetch(t.Context())
	var unknown UnknownCatalogError
	if !errors.As(err, &unknown) {
		t.Fatalf("want UnknownCatalogError, got %v", err)
	}
	_, err = FileIngester{Catalog: "br25", Data: []byte("x")}.Fetch(t.Context())
	if err == nil {
		t.Fatal("want parse error for garbage BR25 payload")
	}
	var ni ErrCatalogNotImplemented
	if errors.As(err, &ni) {
		t.Fatal("BR25 mapping should parse a table, not return ErrCatalogNotImplemented")
	}
}

func TestFileIngesterBR25CSV(t *testing.T) {
	csv := []byte("id,name,a1a3,unit,category\nbr-1,Beton,0.11,kg,Beton\n")
	batch, err := FileIngester{Catalog: "br25", Data: csv, Name: "br25.csv"}.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if batch.CatalogID != model.CatalogDKBR {
		t.Fatalf("catalog=%s", batch.CatalogID)
	}
	if batch.Version != "BR25" {
		t.Fatalf("version=%s", batch.Version)
	}
	if len(batch.Resources) != 1 {
		t.Fatalf("len=%d", len(batch.Resources))
	}
	r := batch.Resources[0]
	if r.CatalogID != model.CatalogDKBR || r.ResourceID != "br-1" {
		t.Fatalf("%+v", r)
	}
	if r.NameSV != "Beton" && r.NameEN != "Beton" {
		t.Fatalf("name sv=%q en=%q", r.NameSV, r.NameEN)
	}
	if r.A1A3 != 0.11 {
		t.Fatalf("a1a3=%v", r.A1A3)
	}
}

func TestFileColumnMapAndBatch(t *testing.T) {
	csv := []byte("foo,gwp,uid\nBeton,0.2,x1\n")
	batch, err := FileIngester{
		Catalog: "dkbr",
		Version: "BR18",
		Data:    csv,
		Name:    "t.csv",
		Map:     map[string]string{"id": "uid", "name": "foo", "a1a3": "gwp"},
	}.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if batch.CatalogID != model.CatalogDKBR || batch.Version != "BR18" {
		t.Fatalf("%+v", batch)
	}
	if batch.Resources[0].ResourceID != "x1" || batch.Resources[0].A1A3 != 0.2 {
		t.Fatalf("%+v", batch.Resources[0])
	}
	prev, err := PreviewFile(csv, "t.csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(prev.Headers) != 3 || prev.Headers[0] != "foo" {
		t.Fatalf("%+v", prev.Headers)
	}
	b, err := ResourceBatch{
		Catalog:   "boverket",
		Version:   "02.07.000",
		Resources: []model.Resource{{ResourceID: "9", NameSV: "X", A1A3: 1, Unit: "kg"}},
	}.Fetch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if b.Resources[0].CatalogID != model.CatalogBoverket || b.Version != "02.07.000" {
		t.Fatalf("%+v", b)
	}
}

func TestHTTPFetcherCatalogID(t *testing.T) {
	var f HTTPFetcher
	if f.CatalogID() != "boverket" {
		t.Fatal(f.CatalogID())
	}
}
