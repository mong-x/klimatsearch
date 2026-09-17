package model

import (
	"strings"
	"testing"
)

func sample() Resource {
	return Resource{
		ResourceID:    "6000000999",
		NameSV:        "Betong",
		NameEN:        "Concrete",
		DescriptionSV: "Generisk betong",
		DescriptionEN: "Generic concrete",
		A1A3:          0.12,
		Unit:          "kg",
		Conversions: map[string]float64{
			"m3": 2400,
			"kg": 1,
		},
		Category: "Betong",
		Version:  "02.07.000",
		RawJSON:  `{"id":"6000000999"}`,
	}
}

func TestOriginBoverketSheet(t *testing.T) {
	r := Resource{CatalogID: CatalogBoverket, ResourceID: "6000000000", CategoryCode: "10"}
	got := r.Origin()
	want := "https://klimatdatabasen.boverket.se/detaljer/10/6000000000"
	if got != want {
		t.Fatalf("got %s", got)
	}
	if r.View("Boverket Klimatdatabas")["origin"] != want {
		t.Fatal("view must include origin")
	}
}

func TestOriginEmptyWithoutCode(t *testing.T) {
	r := Resource{ResourceID: "6000000000"}
	if r.Origin() != "" {
		t.Fatalf("got %s", r.Origin())
	}
	if _, ok := r.View("Boverket Klimatdatabas")["origin"]; ok {
		t.Fatal("view must omit origin when unknown")
	}
}

func TestOriginNotBR25(t *testing.T) {
	r := Resource{CatalogID: CatalogBR25, ResourceID: "x", CategoryCode: "10"}
	if r.Origin() != "" {
		t.Fatalf("br25 should not invent a Boverket URL: %s", r.Origin())
	}
}

func TestContentHashStable(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("hash not stable: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("want 64 hex chars, got %d %q", len(a), a)
	}
}

func TestContentHashMapKeyOrder(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r2 := r
	r2.Conversions = map[string]float64{"kg": 1, "m3": 2400}
	c, err := r2.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if c != a {
		t.Fatalf("map key order affected hash: %s vs %s", a, c)
	}
}

func TestContentHashA1A3Change(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.A1A3 = 0.13
	d, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d == a {
		t.Fatal("changed A1A3 should change hash")
	}
}

func TestContentHashDescriptionChange(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.DescriptionSV = "Annan beskrivning"
	d, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d == a {
		t.Fatal("changed description should change hash")
	}
}

func TestContentHashCategoryChange(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.Category = "Stål"
	d, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d == a {
		t.Fatal("changed Category should change hash")
	}
}

func TestContentHashResourceIDDoesNotChange(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.ResourceID = "other-id"
	r.RawJSON = `{"id":"other-id"}`
	d, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d != a {
		t.Fatal("Resource ID / RawJSON change should not change hash")
	}
}

func TestContentHashNilConversions(t *testing.T) {
	r := Resource{NameSV: "x"}
	h, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Fatal("empty hash")
	}
	r.Conversions = map[string]float64{}
	h2, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if h != h2 {
		t.Fatal("nil and empty conversions should hash the same")
	}
}

func TestContentHashApplicabilityAndSynonyms(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.ApplicabilitySV = "inomhus"
	d, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d == a {
		t.Fatal("changed Applicability should change hash")
	}
	r = sample()
	r.Synonyms = "skivor"
	d, err = r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d == a {
		t.Fatal("changed Synonyms should change hash")
	}
}

func TestContentHashCatalogIDDoesNotChange(t *testing.T) {
	r := sample()
	a, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	r.CatalogID = CatalogBR25
	d, err := r.ContentHash()
	if err != nil {
		t.Fatal(err)
	}
	if d != a {
		t.Fatal("Catalog ID change should not change hash")
	}
}

func TestEmbeddingTextOmitsA1A3(t *testing.T) {
	r := sample()
	r.Category = "Byggskivor"
	r.ApplicabilitySV = "inomhus"
	text := r.EmbeddingText()
	if !strings.Contains(text, "name_sv: Betong") || !strings.Contains(text, "name_en: Concrete") {
		t.Fatalf("labeled names missing: %q", text)
	}
	if !strings.Contains(text, "category: Byggskivor") || !strings.Contains(text, "inomhus") {
		t.Fatalf("embedding text=%q", text)
	}
	if !strings.HasSuffix(strings.TrimSpace(text), "Betong / Concrete") {
		t.Fatalf("names should repeat at end for EOS pooling: %q", text)
	}
	if strings.Contains(text, "Instruct:") {
		t.Fatal("documents must not carry the query instruction prefix")
	}
	if strings.Contains(text, "0.12") {
		t.Fatal("A1A3 must not appear in embedding text")
	}
}

func TestDocID(t *testing.T) {
	if DocID("", "6000000000") != "boverket:6000000000" {
		t.Fatal(DocID("", "6000000000"))
	}
	cat, id := SplitDocID("boverket:6000000000")
	if cat != "boverket" || id != "6000000000" {
		t.Fatalf("%s %s", cat, id)
	}
	cat, id = SplitDocID("6000000000")
	if cat != "" || id != "6000000000" {
		t.Fatalf("bare: %s %s", cat, id)
	}
}
