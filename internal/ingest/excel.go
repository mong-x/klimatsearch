package ingest

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"

	"github.com/mong-x/klimatsearch/internal/model"
)

// ParseExcel reads a Boverket-shaped workbook (SV or EN headers).
// Empty lang detects the header language.
func ParseExcel(data []byte, lang string) (Batch, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Batch{}, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return Batch{}, fmt.Errorf("xlsx has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return Batch{}, fmt.Errorf("xlsx rows: %w", err)
	}
	if len(rows) < 2 {
		return Batch{}, fmt.Errorf("xlsx %s: no data rows", sheets[0])
	}
	return resourcesFromRows(rows, lang, "excel")
}

func resourcesFromRows(rows [][]string, lang, origin string) (Batch, error) {
	if len(rows) < 2 {
		return Batch{}, fmt.Errorf("%s: no data rows", origin)
	}
	if lang == "" {
		lang = detectLang(rows[0])
	}
	idx := headerIndex(rows[0])
	batch := Batch{Origin: origin, CatalogID: model.CatalogBoverket}
	for _, row := range rows[1:] {
		id := cell(row, idx.id)
		if id == "" {
			continue
		}
		r := model.Resource{
			CatalogID:       model.CatalogBoverket,
			ResourceID:      id,
			Category:        cell(row, idx.category),
			Version:         cell(row, idx.version),
			Unit:            declaredUnit(cell(row, idx.unit)),
			Conversions:     map[string]float64{},
			ApplicabilitySV: cell(row, idx.applSV),
			ApplicabilityEN: cell(row, idx.applEN),
			Synonyms:        cell(row, idx.synonyms),
		}
		name := cell(row, idx.name)
		desc := cell(row, idx.desc)
		if lang == "en" {
			r.NameEN = name
			r.DescriptionEN = desc
			if r.ApplicabilityEN == "" {
				r.ApplicabilityEN = cell(row, idx.appl)
			}
		} else {
			r.NameSV = name
			r.DescriptionSV = desc
			if r.ApplicabilitySV == "" {
				r.ApplicabilitySV = cell(row, idx.appl)
			}
		}
		r.A1A3 = parseFloat(cell(row, idx.a1a3))
		if r.A1A3 == 0 {
			r.A1A3 = parseFloat(cell(row, idx.a1a3Energy))
		}
		cf := parseFloat(cell(row, idx.convFactor))
		cu := cell(row, idx.convUnit)
		if cf != 0 && cu != "" {
			r.Conversions[cu] = cf
		}
		if r.Version != "" && batch.Version == "" {
			batch.Version = r.Version
		}
		batch.Resources = append(batch.Resources, r)
	}
	return batch, nil
}

func detectLang(headers []string) string {
	for _, h := range headers {
		n := normHeader(h)
		if n == "product name" || n == "technical description" || n == "resource id" {
			return "en"
		}
	}
	return "sv"
}

// MergeLang joins Swedish and English batches on Resource ID.
func MergeLang(sv, en Batch) Batch {
	byID := map[string]model.Resource{}
	order := make([]string, 0, len(sv.Resources)+len(en.Resources))
	add := func(r model.Resource, fromEN bool) {
		cur, ok := byID[r.ResourceID]
		if !ok {
			byID[r.ResourceID] = r
			order = append(order, r.ResourceID)
			return
		}
		if fromEN {
			if r.NameEN != "" {
				cur.NameEN = r.NameEN
			}
			if r.DescriptionEN != "" {
				cur.DescriptionEN = r.DescriptionEN
			}
			if r.ApplicabilityEN != "" {
				cur.ApplicabilityEN = r.ApplicabilityEN
			}
		} else {
			if r.NameSV != "" {
				cur.NameSV = r.NameSV
			}
			if r.DescriptionSV != "" {
				cur.DescriptionSV = r.DescriptionSV
			}
			if r.ApplicabilitySV != "" {
				cur.ApplicabilitySV = r.ApplicabilitySV
			}
		}
		if cur.Synonyms == "" {
			cur.Synonyms = r.Synonyms
		}
		if cur.A1A3 == 0 {
			cur.A1A3 = r.A1A3
		}
		if cur.Unit == "" {
			cur.Unit = r.Unit
		}
		if cur.Category == "" {
			cur.Category = r.Category
		}
		if cur.Version == "" {
			cur.Version = r.Version
		}
		if cur.CatalogID == "" {
			cur.CatalogID = r.CatalogID
		}
		if cur.Conversions == nil {
			cur.Conversions = map[string]float64{}
		}
		for k, v := range r.Conversions {
			cur.Conversions[k] = v
		}
		cur.Details = model.MergeDetails(cur.Details, r.Details, fromEN)
		byID[r.ResourceID] = cur
	}
	for _, r := range sv.Resources {
		add(r, false)
	}
	for _, r := range en.Resources {
		add(r, true)
	}
	out := Batch{Origin: sv.Origin, Version: sv.Version, CatalogID: sv.CatalogID}
	if out.Origin == "" {
		out.Origin = en.Origin
	}
	if out.Origin == "" {
		out.Origin = "excel"
	}
	if out.Version == "" {
		out.Version = en.Version
	}
	if out.CatalogID == "" {
		out.CatalogID = en.CatalogID
	}
	for _, id := range order {
		out.Resources = append(out.Resources, byID[id])
	}
	return out
}

type colIdx struct {
	id, name, category, version, unit int
	a1a3, a1a3Energy                  int
	desc, convFactor, convUnit        int
	appl, applSV, applEN, synonyms    int
}

func headerIndex(headers []string) colIdx {
	idx := colIdx{id: -1, name: -1, category: -1, version: -1, unit: -1, a1a3: -1, a1a3Energy: -1, desc: -1, convFactor: -1, convUnit: -1, appl: -1, applSV: -1, applEN: -1, synonyms: -1}
	for i, h := range headers {
		n := normHeader(h)
		switch {
		case n == "resurs-id" || n == "resource id" || n == "resourceid" || n == "id":
			idx.id = i
		case n == "produktnamn" || n == "product name" || n == "name" || n == "namn":
			idx.name = i
		case n == "kategori" || n == "category":
			idx.category = i
		case n == "version":
			idx.version = i
		case strings.Contains(n, "enhet for klimat") || strings.Contains(n, "unit for climate"):
			idx.unit = i
		case strings.Contains(n, "typiskt varde") || strings.Contains(n, "typical value") && strings.Contains(n, "a1-a3"):
			if !strings.Contains(n, "energy") && !strings.Contains(n, "energislag") {
				idx.a1a3 = i
			}
		case strings.Contains(n, "energislagets klimat") || (strings.Contains(n, "energy product") && strings.Contains(n, "typical")):
			idx.a1a3Energy = i
		case n == "teknisk beskrivning" || n == "technical description":
			idx.desc = i
		case n == "omrakningsfaktor" || n == "conversion factor":
			idx.convFactor = i
		case strings.Contains(n, "enhet for omrakning") || n == "unit for conversion factor":
			idx.convUnit = i
		case strings.Contains(n, "anvandningsomrade") || n == "use of product" || strings.Contains(n, "technological applicability"):
			idx.appl = i
		case n == "synonyms" || n == "synonymer":
			idx.synonyms = i
		}
	}
	return idx
}

func normHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "å", "a")
	s = strings.ReplaceAll(s, "ä", "a")
	s = strings.ReplaceAll(s, "ö", "o")
	s = strings.ReplaceAll(s, "é", "e")
	return s
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// declaredUnit extracts "kg" from "kg CO₂e/kg".
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
