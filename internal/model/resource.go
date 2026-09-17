package model

import "strings"

// Resource is a generic construction product or energy carrier from Klimatdatabas.
type Resource struct {
	ResourceID    string
	NameSV        string
	NameEN        string
	DescriptionSV string
	DescriptionEN string
	A1A3          float64
	Unit          string
	Conversions   map[string]float64
	Category      string
	ContentHash   string
	Version       string
	RawJSON       string
}

// EmbeddingText is the SV+EN document stored as a vector.
func (r Resource) EmbeddingText() string {
	parts := []string{r.NameSV, r.NameEN, r.DescriptionSV, r.DescriptionEN}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}
