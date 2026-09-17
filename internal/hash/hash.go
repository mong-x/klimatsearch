package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/mong-x/klimatsearch/internal/model"
)

// Content returns the SHA-256 hex digest of a Resource's canonical fields.
// Keys are sorted by encoding/json map encoding.
func Content(r model.Resource) (string, error) {
	conversions := r.Conversions
	if conversions == nil {
		conversions = map[string]float64{}
	}
	payload := map[string]any{
		"a1a3":        r.A1A3,
		"conversions": conversions,
		"name_en":     r.NameEN,
		"name_sv":     r.NameSV,
		"resource_id": r.ResourceID,
		"unit":        r.Unit,
		"version":     r.Version,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("canonical json: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
