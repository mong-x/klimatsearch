package model

import "testing"

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
