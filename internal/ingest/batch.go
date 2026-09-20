package ingest

import (
	"context"
	"fmt"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// ResourceBatch is already-mapped Resources for the Runner.
type ResourceBatch struct {
	Catalog   string           `json:"catalog"`
	Version   string           `json:"version"`
	Resources []model.Resource `json:"-"`
	Items     []ResourceJSON   `json:"resources"`
}

// ResourceJSON is the fill-in schema for POST /admin/resources.
type ResourceJSON struct {
	ID          string  `json:"id"`
	Catalog     string  `json:"catalog"`
	Name        string  `json:"name"`
	NameSV      string  `json:"name_sv"`
	NameEN      string  `json:"name_en"`
	A1A3        float64 `json:"a1a3"`
	Unit        string  `json:"unit"`
	Category    string  `json:"category"`
	Version     string  `json:"version"`
	Description string  `json:"description"`
}

func (b ResourceBatch) CatalogID() string {
	return model.NormalizeCatalogID(b.Catalog)
}

func (b ResourceBatch) Fetch(context.Context) (Batch, error) {
	raw := strings.TrimSpace(b.Catalog)
	catalog := model.NormalizeCatalogID(raw)
	if raw == "" && len(b.Resources) == 0 {
		return Batch{}, fmt.Errorf("catalog is required")
	}
	out := Batch{Origin: "batch", CatalogID: catalog, Version: strings.TrimSpace(b.Version)}
	if out.Version == "" {
		out.Version = model.PublicationForAlias(raw)
	}
	items := b.Resources
	if len(items) == 0 {
		for _, j := range b.Items {
			items = append(items, j.Resource(catalog, out.Version))
		}
	}
	for _, r := range items {
		if r.CatalogID == "" {
			r.CatalogID = catalog
		} else {
			r.CatalogID = model.NormalizeCatalogID(r.CatalogID)
		}
		if r.Version == "" {
			r.Version = out.Version
		}
		if r.ResourceID == "" {
			continue
		}
		out.Resources = append(out.Resources, r)
	}
	if len(out.Resources) == 0 {
		return Batch{}, fmt.Errorf("no resources")
	}
	if out.CatalogID == "" {
		out.CatalogID = out.Resources[0].CatalogID
	}
	return out, nil
}

func (j ResourceJSON) Resource(catalog, version string) model.Resource {
	r := model.Resource{
		CatalogID:  model.NormalizeCatalogID(first(j.Catalog, catalog)),
		ResourceID: strings.TrimSpace(j.ID),
		NameSV:     first(j.NameSV, j.Name),
		NameEN:     j.NameEN,
		A1A3:       j.A1A3,
		Unit:       j.Unit,
		Category:   j.Category,
		Version:    first(j.Version, version),
	}
	if j.Description != "" {
		r.DescriptionSV = j.Description
	}
	return r
}

func first(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}
