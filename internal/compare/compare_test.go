package compare

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mong-x/klimatsearch/internal/model"
	"github.com/mong-x/klimatsearch/internal/store"
)

type fakeGetter map[string]*model.Resource

func (f fakeGetter) Get(_ context.Context, id string) (*model.Resource, error) {
	r, ok := f[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return r, nil
}

func res(id string, a1a3 float64, unit string, conv map[string]float64) *model.Resource {
	return &model.Resource{ResourceID: id, NameSV: id, A1A3: a1a3, Unit: unit, Conversions: conv}
}

func TestResources(t *testing.T) {
	g := fakeGetter{
		"a": res("a", 0.12, "kg", map[string]float64{"kg/m³": 2400}),
		"b": res("b", 1.5, "kg", map[string]float64{"kg/m³": 7800}),
	}
	cmp, err := Resources(t.Context(), g, "a", "b", "Boverket Klimatdatabas", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cmp.LowerImpactID != "a" || cmp.Unit != "kg" || cmp.Incomparable {
		t.Fatalf("cmp=%+v", cmp)
	}

	cmp, err = Resources(t.Context(), g, "a", "b", "Boverket Klimatdatabas", "m³", "")
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Unit != "m³" {
		t.Fatalf("explicit unit via Conversion: %s", cmp.Unit)
	}

	if _, err = Resources(t.Context(), g, "a", "b", "s", "m²", ""); err == nil {
		t.Fatal("explicit missing unit must error")
	}

	if _, err = Resources(t.Context(), g, "a", "b", "s", "", "nope"); err == nil {
		t.Fatal("unknown impact must error")
	}

	for _, tc := range []struct {
		idA, idB, wantSide string
	}{
		{"missing", "b", "a"},
		{"a", "missing", "b"},
	} {
		_, err = Resources(t.Context(), g, tc.idA, tc.idB, "s", "", "")
		var side *SideError
		if !errors.As(err, &side) {
			t.Fatalf("%s/%s: got %T %v, want SideError", tc.idA, tc.idB, err, err)
		}
		if side.Side != tc.wantSide {
			t.Fatalf("side=%s, want %s", side.Side, tc.wantSide)
		}
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("SideError must unwrap to the store sentinel: %v", err)
		}
	}
}

func TestSideErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("boom")
	side := &SideError{Side: "a", Err: inner}
	if side.Error() != "boom" {
		t.Fatalf("Error()=%q, want the inner text with no prefix", side.Error())
	}
	if !errors.Is(side, inner) {
		t.Fatal("errors.Is must chain through SideError")
	}
}
