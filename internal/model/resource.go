package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const BoverketOriginBase = "https://klimatdatabasen.boverket.se"

const (
	CatalogBoverket = "boverket"
	CatalogBR25     = "br25"
)

// Ranked is a Resource at a 1-based position in an FTS or KNN list.
type Ranked struct {
	Resource Resource
	Rank     int
}

// Resource is a generic construction product or energy carrier in a Catalog.
type Resource struct {
	CatalogID       string
	ResourceID      string
	NameSV          string
	NameEN          string
	DescriptionSV   string
	DescriptionEN   string
	ApplicabilitySV string
	ApplicabilityEN string
	Synonyms        string
	A1A3            float64
	Unit            string // Declared unit
	Conversions     map[string]float64
	Category        string
	CategoryCode    string // Boverket Categories[].Code, used to route to the official sheet
	Hash            string // persisted ContentHash
	Version         string // DatasetVersion
	RawJSON         string
}

// Origin is the Catalog's own page for this Resource, if we can form one.
// Boverket's API has no per-id GET; the public sheet is klimatdatabasen.boverket.se/detaljer/{code}/{id}.
func (r Resource) Origin() string {
	cat := r.CatalogID
	if cat == "" {
		cat = CatalogBoverket
	}
	if cat != CatalogBoverket {
		return ""
	}
	code := strings.TrimSpace(r.CategoryCode)
	id := strings.TrimSpace(r.ResourceID)
	if code == "" || id == "" {
		return ""
	}
	return BoverketOriginBase + "/detaljer/" + url.PathEscape(code) + "/" + url.PathEscape(id)
}

// DocID is sqlite-vec's single primary key: catalog_id:resource_id.
func DocID(catalogID, resourceID string) string {
	if catalogID == "" {
		catalogID = CatalogBoverket
	}
	return catalogID + ":" + resourceID
}

// SplitDocID splits a prefixed id. Bare ids return catalog "".
func SplitDocID(id string) (catalogID, resourceID string) {
	id = strings.TrimSpace(id)
	catalogID, resourceID, ok := strings.Cut(id, ":")
	if !ok || catalogID == "" || resourceID == "" {
		return "", id
	}
	return catalogID, resourceID
}

// DocID returns this Resource's prefixed document id.
func (r Resource) DocID() string {
	return DocID(r.CatalogID, r.ResourceID)
}

// EmbeddingText is the labeled bilingual document stored as a vector.
// Names are repeated at the end so last-token/EOS pooling sees sibling modifiers.
// No A1A3 numbers, no instruction prefix.
func (r Resource) EmbeddingText() string {
	var b strings.Builder
	write := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s)
	}
	if v := strings.TrimSpace(r.NameSV); v != "" {
		write("name_sv: " + v)
	}
	if v := strings.TrimSpace(r.NameEN); v != "" {
		write("name_en: " + v)
	}
	if v := strings.TrimSpace(r.Category); v != "" {
		write("category: " + v)
	}
	if v := strings.TrimSpace(r.Unit); v != "" {
		write("unit: " + v)
	}
	write(r.DescriptionSV)
	write(r.DescriptionEN)
	write(r.ApplicabilitySV)
	write(r.ApplicabilityEN)
	switch {
	case strings.TrimSpace(r.NameSV) != "" && strings.TrimSpace(r.NameEN) != "":
		write(strings.TrimSpace(r.NameSV) + " / " + strings.TrimSpace(r.NameEN))
	case strings.TrimSpace(r.NameSV) != "":
		write(r.NameSV)
	default:
		write(r.NameEN)
	}
	return b.String()
}

// ContentHash is the SHA-256 hex digest of the Resource's canonical content.
// Resource ID, Catalog ID, and RawJSON are identity/provenance, not content.
// encoding/json sorts map keys, so Conversion key insertion order does not matter.
func (r Resource) ContentHash() (string, error) {
	conversions := r.Conversions
	if conversions == nil {
		conversions = map[string]float64{}
	}
	payload := map[string]any{
		"a1a3":             r.A1A3,
		"applicability_en": r.ApplicabilityEN,
		"applicability_sv": r.ApplicabilitySV,
		"category":         r.Category,
		"category_code":    r.CategoryCode,
		"conversions":      conversions,
		"description_en":   r.DescriptionEN,
		"description_sv":   r.DescriptionSV,
		"name_en":          r.NameEN,
		"name_sv":          r.NameSV,
		"synonyms":         r.Synonyms,
		"unit":             r.Unit,
		"version":          r.Version,
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
	m := map[string]any{
		"id":               r.ResourceID,
		"catalog":          r.CatalogID,
		"name_sv":          r.NameSV,
		"name_en":          r.NameEN,
		"description_sv":   r.DescriptionSV,
		"description_en":   r.DescriptionEN,
		"applicability_sv": r.ApplicabilitySV,
		"applicability_en": r.ApplicabilityEN,
		"synonyms":         r.Synonyms,
		"a1a3":             r.A1A3,
		"unit":             r.Unit,
		"conversions":      conv,
		"category":         r.Category,
		"version":          r.Version,
		"source":           attribution,
	}
	if r.CategoryCode != "" {
		m["category_code"] = r.CategoryCode
	}
	if o := r.Origin(); o != "" {
		m["origin"] = o
	}
	return m
}
