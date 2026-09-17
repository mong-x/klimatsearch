package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Ranked is a Resource at a 1-based position in an FTS or KNN list.
type Ranked struct {
	Resource Resource
	Rank     int
}

// Resource is a generic construction product or energy carrier from Klimatdatabas.
type Resource struct {
	ResourceID    string
	NameSV        string
	NameEN        string
	DescriptionSV string
	DescriptionEN string
	A1A3          float64
	Unit          string // Declared unit
	Conversions   map[string]float64
	Category      string
	Hash          string // persisted ContentHash
	Version       string // DatasetVersion
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

// ContentHash is the SHA-256 hex digest of the Resource's canonical content.
// Resource ID and RawJSON are identity/provenance, not content.
// encoding/json sorts map keys, so Conversion key insertion order does not matter.
func (r Resource) ContentHash() (string, error) {
	conversions := r.Conversions
	if conversions == nil {
		conversions = map[string]float64{}
	}
	payload := map[string]any{
		"a1a3":           r.A1A3,
		"category":       r.Category,
		"conversions":    conversions,
		"description_en": r.DescriptionEN,
		"description_sv": r.DescriptionSV,
		"name_en":        r.NameEN,
		"name_sv":        r.NameSV,
		"unit":           r.Unit,
		"version":        r.Version,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("canonical json: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// View is the Resource JSON object used by HTTP and MCP.
func (r Resource) View(attribution string) map[string]any {
	conv := r.Conversions
	if conv == nil {
		conv = map[string]float64{}
	}
	return map[string]any{
		"id":             r.ResourceID,
		"name_sv":        r.NameSV,
		"name_en":        r.NameEN,
		"description_sv": r.DescriptionSV,
		"description_en": r.DescriptionEN,
		"a1a3":           r.A1A3,
		"unit":           r.Unit,
		"conversions":    conv,
		"category":       r.Category,
		"version":        r.Version,
		"source":         attribution,
	}
}
