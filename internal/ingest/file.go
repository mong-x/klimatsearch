package ingest

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/mong-x/klimatsearch/internal/model"
)

// FileIngester maps an uploaded workbook or CSV into one Catalog.
type FileIngester struct {
	Catalog string
	Version string
	Data    []byte
	Name    string
	Map     map[string]string // Resource field → file header
}

func (f FileIngester) CatalogID() string {
	return model.NormalizeCatalogID(f.Catalog)
}

func (f FileIngester) Fetch(context.Context) (Batch, error) {
	raw := strings.TrimSpace(f.Catalog)
	catalog := model.NormalizeCatalogID(raw)
	if raw == "" {
		return Batch{}, UnknownCatalogError{Catalog: f.Catalog}
	}
	switch catalog {
	case model.CatalogBoverket, model.CatalogDKBR:
	default:
		return Batch{}, UnknownCatalogError{Catalog: f.Catalog}
	}
	batch, err := parseTableFile(f.Data, f.Name, catalog, f.Map)
	if err != nil {
		return Batch{}, err
	}
	batch.Origin = "file"
	batch.CatalogID = catalog
	if catalog == model.CatalogBoverket {
		stampBoverket(&batch)
	}
	ver := strings.TrimSpace(f.Version)
	if ver == "" {
		ver = model.PublicationForAlias(raw)
	}
	if ver != "" {
		batch.Version = ver
	}
	return batch, nil
}

func parseTableFile(data []byte, name, catalog string, fmap map[string]string) (Batch, error) {
	lower := strings.ToLower(name)
	var (
		rows [][]string
		err  error
	)
	switch {
	case looksXLSX(data) || strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls"):
		rows, err = excelRows(data)
	case strings.HasSuffix(lower, ".csv") || looksCSV(data):
		rows, err = csvRows(data)
	default:
		rows, err = csvRows(data)
	}
	if err != nil {
		return Batch{}, err
	}
	batch, err := resourcesFromRows(rows, "", "file", catalog, fmap)
	if err != nil {
		return Batch{}, err
	}
	if len(batch.Resources) == 0 {
		return Batch{}, fmt.Errorf("no rows with an id (need a header like id, resource id, or a column map)")
	}
	return batch, nil
}

func csvRows(data []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.ReuseRecord = false
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv: no data rows")
	}
	return rows, nil
}

func looksXLSX(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4b && data[2] == 0x03 && data[3] == 0x04
}

func looksCSV(data []byte) bool {
	s := string(bytes.TrimSpace(data))
	return strings.Contains(s, ",") && !looksXLSX(data)
}

// ParseCSV reads a Boverket-shaped CSV using the same headers as Excel.
func ParseCSV(data []byte, lang string) (Batch, error) {
	rows, err := csvRows(data)
	if err != nil {
		return Batch{}, err
	}
	return resourcesFromRows(rows, lang, "file", model.CatalogBoverket, nil)
}

// SchemaField is a Resource field a Column map may target.
type SchemaField struct {
	Field string
	Label string
}

// ResourceSchema is the fill-in surface for a file Column map.
func ResourceSchema() []SchemaField {
	return []SchemaField{
		{Field: "id", Label: "Resource ID"},
		{Field: "name", Label: "Name"},
		{Field: "name_en", Label: "Name (en)"},
		{Field: "a1a3", Label: "Typical A1–A3"},
		{Field: "unit", Label: "Declared unit"},
		{Field: "category", Label: "Category"},
		{Field: "version", Label: "DatasetVersion"},
		{Field: "description", Label: "Description"},
		{Field: "a4", Label: "A4"},
		{Field: "a5_1", Label: "A5.1"},
	}
}

// Preview is headers and sample rows so a caller can fill a Column map.
type Preview struct {
	Headers []string      `json:"headers"`
	Sample  [][]string    `json:"sample"`
	Schema  []SchemaField `json:"schema"`
}

// PreviewFile reads headers without ingesting.
func PreviewFile(data []byte, name string) (Preview, error) {
	lower := strings.ToLower(name)
	var (
		rows [][]string
		err  error
	)
	switch {
	case looksXLSX(data) || strings.HasSuffix(lower, ".xlsx"):
		rows, err = excelRows(data)
	default:
		rows, err = csvRows(data)
	}
	if err != nil {
		return Preview{}, err
	}
	p := Preview{Headers: rows[0], Schema: ResourceSchema()}
	max := 6
	if len(rows) < max {
		max = len(rows)
	}
	p.Sample = rows[1:max]
	return p, nil
}

// ParseMapping reads form keys map.id, map.a1a3, …
func ParseMapping(form map[string][]string) map[string]string {
	out := map[string]string{}
	for k, vs := range form {
		if !strings.HasPrefix(k, "map.") || len(vs) == 0 {
			continue
		}
		field := strings.TrimPrefix(k, "map.")
		val := strings.TrimSpace(vs[0])
		if field == "" || val == "" {
			continue
		}
		out[field] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// UnknownCatalogError is a Catalog ID the File Ingester does not know.
type UnknownCatalogError struct {
	Catalog string
}

func (e UnknownCatalogError) Error() string {
	return fmt.Sprintf("unknown catalog %q", e.Catalog)
}

// ErrCatalogNotImplemented is a known Catalog without a file mapping yet.
type ErrCatalogNotImplemented struct {
	Catalog string
}

func (e ErrCatalogNotImplemented) Error() string {
	return fmt.Sprintf("catalog %q mapping is not implemented", e.Catalog)
}
