package hash

import (
	"testing"

	"github.com/mong-x/klimatsearch/internal/model"
)

func TestContentStable(t *testing.T) {
	r := model.Resource{
		ResourceID: "6000000999",
		NameSV:     "Betong",
		NameEN:     "Concrete",
		A1A3:       0.12,
		Unit:       "kg",
		Conversions: map[string]float64{
			"m3": 2400,
			"kg": 1,
		},
		Version: "02.07.000",
	}
	a, err := Content(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Content(r)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("hash not stable: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("want 64 hex chars, got %d %q", len(a), a)
	}

	r2 := r
	r2.Conversions = map[string]float64{"kg": 1, "m3": 2400}
	c, err := Content(r2)
	if err != nil {
		t.Fatal(err)
	}
	if c != a {
		t.Fatalf("map key order affected hash: %s vs %s", a, c)
	}

	r3 := r
	r3.A1A3 = 0.13
	d, err := Content(r3)
	if err != nil {
		t.Fatal(err)
	}
	if d == a {
		t.Fatal("changed A1A3 should change hash")
	}
}

func TestContentNilConversions(t *testing.T) {
	r := model.Resource{ResourceID: "1", NameSV: "x"}
	h, err := Content(r)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Fatal("empty hash")
	}
}
