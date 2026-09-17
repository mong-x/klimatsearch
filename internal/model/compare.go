package model

import (
	"fmt"
	"sort"
)

// Comparison is A1A3 of two Resources expressed in one shared unit.
type Comparison struct {
	A             map[string]any
	B             map[string]any
	Attribution   string
	Unit          string
	DeltaA1A3     float64
	LowerImpactID string
	Incomparable  bool
}

// ErrUnitUnavailable is returned when an explicit unit cannot be applied to both Resources.
type ErrUnitUnavailable struct {
	Unit string
	ID   string
}

func (e ErrUnitUnavailable) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("unit %q cannot be applied to resource %s", e.Unit, e.ID)
	}
	return fmt.Sprintf("unit %q cannot be applied to both resources", e.Unit)
}

// Compare restates A1A3 of a and b in one unit.
//
// A1A3 is per Declared unit D. Boverket conversions are typically kg per other
// unit (e.g. kg/m³). If comparing in unit U: if D==U use A1A3; if Conversion[U]
// exists, value = A1A3 * Conversion[U]. If Declared unit is kg and
// Conversion["kg/m³"]=2400, A1A3 per m³ = A1A3 * 2400.
//
// With no unit: matching Declared units, else both convertible to kg, else
// exactly one shared Conversion key. Otherwise incomparable (no numeric delta).
// An explicit unit that cannot apply is an error, never silent incomparable.
func Compare(a, b Resource, attribution, unit string) (Comparison, error) {
	out := Comparison{
		A:           a.View(attribution),
		B:           b.View(attribution),
		Attribution: attribution,
	}
	if unit != "" {
		va, okA := a1a3InUnit(a, unit)
		if !okA {
			return Comparison{}, ErrUnitUnavailable{Unit: unit, ID: a.ResourceID}
		}
		vb, okB := a1a3InUnit(b, unit)
		if !okB {
			return Comparison{}, ErrUnitUnavailable{Unit: unit, ID: b.ResourceID}
		}
		return finishCompare(out, a, b, unit, va, vb), nil
	}

	if a.Unit != "" && a.Unit == b.Unit {
		return finishCompare(out, a, b, a.Unit, a.A1A3, b.A1A3), nil
	}
	if convertsToKg(a) && convertsToKg(b) {
		va, _ := a1a3InUnit(a, "kg")
		vb, _ := a1a3InUnit(b, "kg")
		return finishCompare(out, a, b, "kg", va, vb), nil
	}
	shared := sharedConversionKeys(a, b)
	if len(shared) == 1 {
		u := shared[0]
		va, okA := a1a3InUnit(a, u)
		vb, okB := a1a3InUnit(b, u)
		if okA && okB {
			return finishCompare(out, a, b, u, va, vb), nil
		}
	}
	out.Incomparable = true
	return out, nil
}

func finishCompare(out Comparison, a, b Resource, unit string, va, vb float64) Comparison {
	out.Unit = unit
	out.DeltaA1A3 = va - vb
	out.LowerImpactID = a.ResourceID
	if vb < va {
		out.LowerImpactID = b.ResourceID
	}
	return out
}

// a1a3InUnit restates A1A3 in unit u.
// value = A1A3 is per Declared unit. Conversion factors multiply A1A3 to
// express it per the target unit (Boverket: kg per m³ etc.).
func a1a3InUnit(r Resource, u string) (float64, bool) {
	if u == "" {
		return 0, false
	}
	if r.Unit == u {
		return r.A1A3, true
	}
	if r.Conversions != nil {
		if f, ok := r.Conversions[u]; ok {
			return r.A1A3 * f, true
		}
		// Composite key "{declared}/{target}", e.g. kg/m³ when asking for m³.
		if f, ok := r.Conversions[r.Unit+"/"+u]; ok {
			return r.A1A3 * f, true
		}
	}
	return 0, false
}

func convertsToKg(r Resource) bool {
	_, ok := a1a3InUnit(r, "kg")
	return ok
}

func sharedConversionKeys(a, b Resource) []string {
	if a.Conversions == nil || b.Conversions == nil {
		return nil
	}
	var keys []string
	for k := range a.Conversions {
		if _, ok := b.Conversions[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}
