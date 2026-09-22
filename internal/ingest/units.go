package ingest

import (
	"strings"
	"unicode"
)

// declaredUnit extracts the Declared unit from a GWP-shaped unit string:
// "kg CO₂e/kg" → "kg", "kg CO2 eq./kWh" → "kWh", bare "kWh" → "kWh".
func declaredUnit(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.LastIndex(s, "/"); i >= 0 && i+1 < len(s) {
		return strings.TrimSpace(s[i+1:])
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || r == '²' || r == '³' || r == '2' || r == '3' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			break
		}
	}
	return b.String()
}

// gwpUnit restates a source GWP unit in one canonical spelling so the JSON
// API ("kg CO2 eq./kg") and the Excel fallback ("kg CO₂e/kg") produce the
// same Details.GWPUnit - and therefore the same ContentHash. GWP is by
// definition kg CO2e per Declared unit, so nothing meaningful is lost.
func gwpUnit(s string) string {
	d := declaredUnit(s)
	if d == "" {
		return ""
	}
	return "kg CO2e/" + d
}
