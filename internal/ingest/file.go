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
	Data    []byte
	Name    string
}

func (f FileIngester) CatalogID() string { return f.Catalog }

func (f FileIngester) Fetch(context.Context) (Batch, error) {
	catalog := strings.ToLower(strings.TrimSpace(f.Catalog))
	switch catalog {
	case model.CatalogBoverket:
		batch, err := parseBoverketFile(f.Data, f.Name)
		if err != nil {
			return Batch{}, err
		}
		batch.Origin = "file"
		batch.CatalogID = model.CatalogBoverket
		stampBoverket(&batch)
		return batch, nil
	case model.CatalogBR25:
		return Batch{}, ErrCatalogNotImplemented{Catalog: model.CatalogBR25}
	case "":
		return Batch{}, UnknownCatalogError{Catalog: f.Catalog}
	default:
		return Batch{}, UnknownCatalogError{Catalog: f.Catalog}
	}
}

func parseBoverketFile(data []byte, name string) (Batch, error) {
	lower := strings.ToLower(name)
	if looksXLSX(data) || strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls") {
		return ParseExcel(data, "")
	}
	if strings.HasSuffix(lower, ".csv") || looksCSV(data) {
		return ParseCSV(data, "")
	}
	if looksXLSX(data) {
		return ParseExcel(data, "")
	}
	return ParseCSV(data, "")
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
	r := csv.NewReader(bytes.NewReader(data))
	r.ReuseRecord = false
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return Batch{}, fmt.Errorf("csv: %w", err)
	}
	if len(rows) < 2 {
		return Batch{}, fmt.Errorf("csv: no data rows")
	}
	return resourcesFromRows(rows, lang, "file")
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
