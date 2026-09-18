package model

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ImpactTypical      = "typical"
	ImpactConservative = "conservative"
	ImpactA4           = "a4"
	ImpactA51          = "a5_1"
)

// Comparison is A1A3 of two Resources expressed in one shared unit.
type Comparison struct {
	A             map[string]any
	B             map[string]any
	Attribution   string
	Unit          string
	Impact        string
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
func Compare(a, b Resource, attribution, unit, impact string) (Comparison, error) {
	imp, err := NormalizeImpact(impact)
	if err != nil {
		return Comparison{}, err
	}
	out := Comparison{
		A:           a.View(attribution),
		B:           b.View(attribution),
		Attribution: attribution,
		Impact:      imp,
	}
	if unit != "" {
		va, okA := impactInUnit(a, unit, imp)
		if !okA {
			return Comparison{}, ErrUnitUnavailable{Unit: unit, ID: a.ResourceID}
		}
		vb, okB := impactInUnit(b, unit, imp)
		if !okB {
			return Comparison{}, ErrUnitUnavailable{Unit: unit, ID: b.ResourceID}
		}
		return finishCompare(out, a, b, unit, va, vb), nil
	}

	if a.Unit != "" && a.Unit == b.Unit {
		va, okA := climateValue(a, imp)
		vb, okB := climateValue(b, imp)
		if !okA || !okB {
			out.Incomparable = true
			return out, nil
		}
		return finishCompare(out, a, b, a.Unit, va, vb), nil
	}
	if convertsToKg(a, imp) && convertsToKg(b, imp) {
		va, _ := impactInUnit(a, "kg", imp)
		vb, _ := impactInUnit(b, "kg", imp)
		return finishCompare(out, a, b, "kg", va, vb), nil
	}
	shared := sharedConversionKeys(a, b)
	if len(shared) == 1 {
		u := shared[0]
		va, okA := impactInUnit(a, u, imp)
		vb, okB := impactInUnit(b, u, imp)
		if okA && okB {
			return finishCompare(out, a, b, u, va, vb), nil
		}
	}
	out.Incomparable = true
	return out, nil
}

// View is the Comparison JSON used by REST and MCP.
func (c Comparison) View() map[string]any {
	return map[string]any{
		"source":          c.Attribution,
		"a":               c.A,
		"b":               c.B,
		"unit":            c.Unit,
		"impact":          c.Impact,
		"delta_a1a3":      c.DeltaA1A3,
		"lower_impact_id": c.LowerImpactID,
		"incomparable":    c.Incomparable,
	}
}

// ErrImpactUnknown is returned when impact is not typical|conservative|a4|a5_1.
type ErrImpactUnknown struct{ Impact string }

func (e ErrImpactUnknown) Error() string {
	return fmt.Sprintf("unknown impact %q (typical|conservative|a4|a5_1)", e.Impact)
}

func NormalizeImpact(impact string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(impact))
	switch s {
	case "", ImpactTypical, "a1a3":
		return ImpactTypical, nil
	case ImpactConservative:
		return ImpactConservative, nil
	case ImpactA4:
		return ImpactA4, nil
	case ImpactA51, "a51", "a5.1":
		return ImpactA51, nil
	default:
		return "", ErrImpactUnknown{Impact: impact}
	}
}

func climateValue(r Resource, impact string) (float64, bool) {
	switch impact {
	case ImpactTypical:
		return r.A1A3, true
	case ImpactConservative:
		v := r.Details.A1A3Conservative
		if v == 0 && r.A1A3 != 0 {
			return 0, false
		}
		return v, true
	case ImpactA4:
		if r.Details.A4 == nil {
			return 0, false
		}
		return *r.Details.A4, true
	case ImpactA51:
		if r.Details.A51 == nil {
			return 0, false
		}
		return *r.Details.A51, true
	default:
		return 0, false
	}
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

func impactInUnit(r Resource, u, impact string) (float64, bool) {
	base, ok := climateValue(r, impact)
	if !ok || u == "" {
		return 0, false
	}
	if r.Unit == u {
		return base, true
	}
	if r.Conversions != nil {
		if f, ok := r.Conversions[u]; ok {
			return base * f, true
		}
		if f, ok := r.Conversions[r.Unit+"/"+u]; ok {
			return base * f, true
		}
	}
	return 0, false
}

func convertsToKg(r Resource, impact string) bool {
	_, ok := impactInUnit(r, "kg", impact)
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
