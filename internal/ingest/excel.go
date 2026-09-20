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
	rows, err := excelRows(data)
	if err != nil {
		return Batch{}, err
	}
	return resourcesFromRows(rows, lang, "excel", model.CatalogBoverket, nil)
}

func excelRows(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("xlsx rows: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("xlsx %s: no data rows", sheets[0])
	}
	return rows, nil
}

func resourcesFromRows(rows [][]string, lang, origin, catalog string, fmap map[string]string) (Batch, error) {
	if len(rows) < 2 {
		return Batch{}, fmt.Errorf("%s: no data rows", origin)
	}
	if catalog == "" {
		catalog = model.CatalogBoverket
	}
	if lang == "" {
		lang = detectLang(rows[0])
	}
	idx := applyColumnMap(rows[0], headerIndex(rows[0]), fmap)
	batch := Batch{Origin: origin, CatalogID: catalog}
	for _, row := range rows[1:] {
		id := cell(row, idx.id)
		if id == "" {
			continue
		}
		r := model.Resource{
			CatalogID:       catalog,
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
		if en := cell(row, idx.nameEN); en != "" {
			r.NameEN = en
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
		r.Details = detailsFromExcel(row, idx)
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
	id, name, nameEN, category, version, unit int
	a1a3, a1a3Energy                          int
	a1a3Cons, consFactor, waste               int
	a4, a51, biogenic, life, bk04             int
	desc, convFactor, convUnit                int
	appl, applSV, applEN, synonyms            int
}

func detailsFromExcel(row []string, idx colIdx) model.Details {
	d := model.Details{GWPUnit: cell(row, idx.unit)}
	if f := parseFloat(cell(row, idx.a1a3Cons)); f != 0 {
		d.A1A3Conservative = f
	}
	if f := parseFloat(cell(row, idx.consFactor)); f != 0 {
		d.ConservativeFactor = f
	}
	if f := parseFloat(cell(row, idx.waste)); f != 0 {
		d.WasteFactor = f
	}
	if s := cell(row, idx.a4); s != "" {
		v := parseFloat(s)
		d.A4 = &v
	}
	if s := cell(row, idx.a51); s != "" {
		v := parseFloat(s)
		d.A51 = &v
	}
	if s := cell(row, idx.biogenic); s != "" {
		v := parseFloat(s)
		d.BiogenicCarbon = &v
	}
	d.ServiceLife = cell(row, idx.life)
	d.BK04Code = cell(row, idx.bk04)
	return d
}

func headerIndex(headers []string) colIdx {
	idx := colIdx{
		id: -1, name: -1, nameEN: -1, category: -1, version: -1, unit: -1,
		a1a3: -1, a1a3Energy: -1, a1a3Cons: -1, consFactor: -1, waste: -1,
		a4: -1, a51: -1, biogenic: -1, life: -1, bk04: -1,
		desc: -1, convFactor: -1, convUnit: -1, appl: -1, applSV: -1, applEN: -1, synonyms: -1,
	}
	for i, h := range headers {
		n := normHeader(h)
		switch {
		case n == "resurs-id" || n == "resource id" || n == "resourceid" || n == "id" || n == "produkt-id" || n == "ressource-id":
			idx.id = i
		case n == "produktnamn" || n == "product name" || n == "name" || n == "namn" || n == "navn" || n == "produkt":
			idx.name = i
		case n == "kategori" || n == "category" || n == "bygningsdel":
			idx.category = i
		case n == "version":
			idx.version = i
		case n == "unit" || n == "enhet" || n == "enhed":
			if idx.unit < 0 {
				idx.unit = i
			}
		case strings.Contains(n, "enhet for klimat") || strings.Contains(n, "unit for climate"):
			idx.unit = i
		case n == "a1a3" || n == "a1-a3" || n == "gwp" || n == "gwp-a1-a3" || n == "gwp a1-a3":
			if idx.a1a3 < 0 {
				idx.a1a3 = i
			}
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
		case strings.Contains(n, "konservativ") && strings.Contains(n, "a1-a3") && !strings.Contains(n, "faktor"):
			idx.a1a3Cons = i
		case strings.Contains(n, "conservative") && strings.Contains(n, "a1-a3") && strings.Contains(n, "value"):
			idx.a1a3Cons = i
		case strings.Contains(n, "faktor for konservativ") || (strings.Contains(n, "conservative") && strings.Contains(n, "factor")):
			idx.consFactor = i
		case n == "avfallsfaktor" || n == "waste factor":
			idx.waste = i
		case n == "a4" || strings.HasPrefix(n, "a4 ") || (strings.Contains(n, "a4") && strings.Contains(n, "gwp")):
			if idx.a4 < 0 && !strings.Contains(n, "a1") {
				idx.a4 = i
			}
		case strings.Contains(n, "a5.1") || strings.Contains(n, "a5,1"):
			idx.a51 = i
		case strings.Contains(n, "biogen"):
			idx.biogenic = i
		case strings.Contains(n, "referenslivslangd") || strings.Contains(n, "reference service life"):
			idx.life = i
		case strings.Contains(n, "bk04"):
			idx.bk04 = i
		}
	}
	return idx
}

func applyColumnMap(headers []string, idx colIdx, fmap map[string]string) colIdx {
	if len(fmap) == 0 {
		return idx
	}
	pos := map[string]int{}
	for i, h := range headers {
		pos[normHeader(h)] = i
		pos[strings.ToLower(strings.TrimSpace(h))] = i
	}
	set := func(field string, dst *int) {
		h := strings.TrimSpace(fmap[field])
		if h == "" {
			return
		}
		if i, ok := pos[normHeader(h)]; ok {
			*dst = i
			return
		}
		if i, ok := pos[strings.ToLower(h)]; ok {
			*dst = i
		}
	}
	set("id", &idx.id)
	set("name", &idx.name)
	set("name_en", &idx.nameEN)
	set("a1a3", &idx.a1a3)
	set("unit", &idx.unit)
	set("category", &idx.category)
	set("version", &idx.version)
	set("description", &idx.desc)
	set("a4", &idx.a4)
	set("a5_1", &idx.a51)
	return idx
}

func normHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "å", "a")
	s = strings.ReplaceAll(s, "ä", "a")
	s = strings.ReplaceAll(s, "ö", "o")
	s = strings.ReplaceAll(s, "æ", "ae")
	s = strings.ReplaceAll(s, "ø", "o")
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
