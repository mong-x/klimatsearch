package model

import (
	"errors"
	"math"
	"testing"
)

func TestCompareA4(t *testing.T) {
	a4a, a4b := 0.06, 0.10
	a := Resource{ResourceID: "a", A1A3: 0.39, Unit: "kg", Details: Details{A4: &a4a}}
	b := Resource{ResourceID: "b", A1A3: 0.50, Unit: "kg", Details: Details{A4: &a4b}}
	got, err := Compare(a, b, "Boverket Klimatdatabas", "kg", "a4")
	if err != nil {
		t.Fatal(err)
	}
	if got.Impact != ImpactA4 || math.Abs(got.DeltaA1A3-(-0.04)) > 1e-9 || got.LowerImpactID != "a" {
		t.Fatalf("%+v", got)
	}
}

func TestCompareTable(t *testing.T) {
	kgA := Resource{ResourceID: "a", NameSV: "A", A1A3: 0.12, Unit: "kg", Conversions: map[string]float64{"kg/m³": 2400}}
	kgB := Resource{ResourceID: "b", NameSV: "B", A1A3: 1.55, Unit: "kg", Conversions: map[string]float64{"kg/m³": 1800}}
	m3 := Resource{ResourceID: "c", NameSV: "C", A1A3: 200, Unit: "m³"}
	wood := Resource{ResourceID: "d", NameSV: "D", A1A3: 0.4, Unit: "m³", Conversions: map[string]float64{"m²": 0.02}}

	tests := []struct {
		name       string
		a, b       Resource
		unit       string
		wantUnit   string
		wantDelta  float64
		wantLower  string
		wantIncomp bool
		wantErr    bool
	}{
		{
			name:      "same declared unit",
			a:         kgA,
			b:         kgB,
			wantUnit:  "kg",
			wantDelta: 0.12 - 1.55,
			wantLower: "a",
		},
		{
			name:      "convert via kg/m³",
			a:         kgA,
			b:         kgB,
			unit:      "m³",
			wantUnit:  "m³",
			wantDelta: 0.12*2400 - 1.55*1800,
			wantLower: "a",
		},
		{
			name:    "explicit unit missing",
			a:       kgA,
			b:       kgB,
			unit:    "m²",
			wantErr: true,
		},
		{
			name:       "auto incomparable",
			a:          kgA,
			b:          wood,
			wantIncomp: true,
		},
		{
			name:      "declared already kg asked kg",
			a:         kgA,
			b:         kgB,
			unit:      "kg",
			wantUnit:  "kg",
			wantDelta: 0.12 - 1.55,
			wantLower: "a",
		},
		{
			name:       "kg vs m³ no conversion auto incomparable",
			a:          Resource{ResourceID: "a", A1A3: 1, Unit: "kg"},
			b:          m3,
			wantIncomp: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compare(tt.a, tt.b, "Boverket Klimatdatabas", tt.unit, "")
			if tt.wantErr {
				var uerr ErrUnitUnavailable
				if !errors.As(err, &uerr) {
					t.Fatalf("want ErrUnitUnavailable, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Incomparable != tt.wantIncomp {
				t.Fatalf("incomparable=%v want %v", got.Incomparable, tt.wantIncomp)
			}
			if tt.wantIncomp {
				if got.DeltaA1A3 != 0 {
					t.Fatalf("incomparable must not subtract: delta=%v", got.DeltaA1A3)
				}
				return
			}
			if got.Unit != tt.wantUnit {
				t.Fatalf("unit=%q want %q", got.Unit, tt.wantUnit)
			}
			if math.Abs(got.DeltaA1A3-tt.wantDelta) > 1e-9 {
				t.Fatalf("delta=%v want %v", got.DeltaA1A3, tt.wantDelta)
			}
			if got.LowerImpactID != tt.wantLower {
				t.Fatalf("lower=%q want %q", got.LowerImpactID, tt.wantLower)
			}
		})
	}
}

func TestResourceView(t *testing.T) {
	r := Resource{ResourceID: "1", NameSV: "Betong", A1A3: 0.1, Unit: "kg"}
	v := r.View("Boverket Klimatdatabas")
	if v["id"] != "1" || v["source"] != "Boverket Klimatdatabas" {
		t.Fatalf("%v", v)
	}
	if _, ok := v["catalog"]; !ok {
		t.Fatal("view must include catalog")
	}
	if v["conversions"] == nil {
		t.Fatal("conversions should be empty map, not nil")
	}
}
