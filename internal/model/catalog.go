package model

import "strings"

const (
	CatalogBoverket = "boverket"
	CatalogDKBR     = "dkbr"
	// CatalogBR25 is a deprecated alias for CatalogDKBR.
	CatalogBR25 = "br25"
)

// CatalogInfo is one Catalog the process knows.
type CatalogInfo struct {
	ID       string
	Title    string
	FileOnly bool
}

// Catalogs is the registry the console iterates.
func Catalogs() []CatalogInfo {
	return []CatalogInfo{
		{ID: CatalogBoverket, Title: "Boverket Klimatdatabas"},
		{ID: CatalogDKBR, Title: "Danish BR", FileOnly: true},
	}
}

// NormalizeCatalogID maps aliases onto a Catalog ID. Unknown values are lowercased.
func NormalizeCatalogID(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", CatalogBoverket:
		return CatalogBoverket
	case CatalogDKBR, "dk-br", CatalogBR25, "br18":
		return CatalogDKBR
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

// ParseCatalogIDs splits and normalizes Catalog IDs. Empty means all Catalogs.
// A request for dkbr also includes legacy rows stored as br25.
func ParseCatalogIDs(parts []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, p := range parts {
		for _, bit := range strings.Split(p, ",") {
			id := NormalizeCatalogID(strings.TrimSpace(bit))
			if strings.TrimSpace(bit) == "" {
				continue
			}
			add(id)
			if id == CatalogDKBR {
				add(CatalogBR25)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// DatasetVersionMetaKey is meta for one Catalog's DatasetVersion.
func DatasetVersionMetaKey(catalogID string) string {
	return "dataset_version:" + NormalizeCatalogID(catalogID)
}

// PublicationForAlias returns a DatasetVersion when the caller used a year alias.
func PublicationForAlias(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "br25":
		return "BR25"
	case "br18":
		return "BR18"
	default:
		return ""
	}
}
