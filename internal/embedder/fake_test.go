package embedder

import (
	"math"
	"testing"
)

func TestFakeDeterministic(t *testing.T) {
	var f Fake
	a, err := f.Embed("Betong Concrete")
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.Embed("Betong Concrete")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != Dim || len(b) != Dim {
		t.Fatalf("dim: %d %d want %d", len(a), len(b), Dim)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("mismatch at %d: %v vs %v", i, a[i], b[i])
		}
	}
	c, err := f.Embed("Steel")
	if err != nil {
		t.Fatal(err)
	}
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different text produced identical vector")
	}
	var sum float64
	for _, x := range a {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			t.Fatalf("non-finite component %v", x)
		}
		sum += float64(x) * float64(x)
	}
	if math.Abs(sum-1) > 1e-5 {
		t.Fatalf("not unit length: %v", sum)
	}
}
